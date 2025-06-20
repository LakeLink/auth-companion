import json
import aiohttp
import logging

from typing import Optional

from pydantic import BaseModel, Field
from fastapi import HTTPException
from fastapi.responses import StreamingResponse
from starlette.background import BackgroundTask

from open_webui.routers.openai import (
    openai_o_series_handler,
    send_get_request,
    cleanup_response,
)
from open_webui.models.users import Users, UserModel
from open_webui.env import (
    ENV,
    SRC_LOG_LEVELS,
    AIOHTTP_CLIENT_SESSION_SSL,
    AIOHTTP_CLIENT_TIMEOUT,
    AIOHTTP_CLIENT_TIMEOUT_MODEL_LIST,
    ENABLE_FORWARD_USER_INFO_HEADERS,
)
from open_webui.utils.misc import convert_logit_bias_input_to_json

log = logging.getLogger(__name__)
log.setLevel(SRC_LOG_LEVELS["OPENAI"])


class Pipe:
    class Valves(BaseModel):
        API_BASE_URL: str = Field(
            default="https://api.openai.com/v1",
            description="Base URL for accessing OpenAI API endpoints.",
        )
        API_TOKEN_URL: str = Field(
            default="http://companion/open-webui/ensure_token",
            description="Base URL for obtaining API token.",
        )
        TOKEN_NAME: str = Field(default="open-webui")
        TOKEN_GROUP: str = Field(default="open-webui")

        GLOBAL_SHARED_TOKEN: str = Field(
            default="", description="Shared API token for listing models."
        )
        pass

    def __init__(self):
        self.valves = self.Valves()

    async def pipes(self):
        if self.valves.GLOBAL_SHARED_TOKEN:
            try:
                response = await send_get_request(
                    f"{self.valves.API_BASE_URL}/models",
                    self.valves.GLOBAL_SHARED_TOKEN,
                )
                response = (
                    response if isinstance(response, list) else response.get("data", [])
                )

                log.debug(f"pipes:response {response}")

                return [
                    {
                        **model,
                        "name": model.get("name", model["id"]),
                    }
                    for model in response
                    if model.get("id")
                    or model.get("name")
                    and (
                        not any(
                            name in model["id"]
                            for name in [
                                "babbage",
                                "dall-e",
                                "davinci",
                                "embedding",
                                "tts",
                                "whisper",
                            ]
                        )
                    )
                ]
            except Exception as e:
                raise
        else:
            return []

    async def pipe(self, body: dict, __user__: dict):
        user = Users.get_user_by_id(__user__["id"])
        if not user.oauth_sub.startswith("oidc@"):
            raise HTTPException(
                status_code=403,
                detail="You are not registered through LakeLink ZITADEL.",
            )

        oidc_user_id = user.oauth_sub.split("@")[1]
        status, token = await self.obtain_user_api_key(oidc_user_id)
        if status == 404:
            raise HTTPException(
                status_code=404,
                detail="User not found at LakeLink AI Aggregator.\nPlease first login using OIDC at https://ai.lklk.tech.",
            )
        elif status != 200:
            raise HTTPException(
                status_code=500,
                detail=f"An error occured while obtaining API key: {token}",
            )

        return await self.forward(self.valves.API_BASE_URL, token, body)

    async def forward(self, url: str, key: str, payload: dict):
        # open_webui/functions.py get_pipe_id
        # def split_pipe_id(form_data: dict) -> tuple[Optional[str], str]:
        #     model = form_data["model"]
        #     pipe_id = None
        #     if "." in model:
        #         pipe_id, model = model.split(".", 1)
        #     return pipe_id, model
        
        _, model= payload["model"].split(".", 1)
        payload["model"] = model

        # Check if model is from "o" series
        is_o_series = payload["model"].lower().startswith(("o1", "o3", "o4"))
        if is_o_series:
            payload = openai_o_series_handler(payload)

        # Convert the modified body back to JSON
        if "logit_bias" in payload:
            payload["logit_bias"] = json.loads(
                convert_logit_bias_input_to_json(payload["logit_bias"])
            )

        headers = {
            "Content-Type": "application/json",
            **(
                {
                    "HTTP-Referer": "https://openwebui.com/",
                    "X-Title": "Open WebUI",
                }
                if "openrouter.ai" in url
                else {}
            ),
            # **(
            #     {
            #         "X-OpenWebUI-User-Name": user.name,
            #         "X-OpenWebUI-User-Id": user.id,
            #         "X-OpenWebUI-User-Email": user.email,
            #         "X-OpenWebUI-User-Role": user.role,
            #     }
            #     if ENABLE_FORWARD_USER_INFO_HEADERS
            #     else {}
            # ),
        }

        request_url = f"{url}/chat/completions"
        headers["Authorization"] = f"Bearer {key}"

        payload = json.dumps(payload)

        r = None
        session = None
        streaming = False
        response = None

        try:
            session = aiohttp.ClientSession(
                trust_env=True,
                timeout=aiohttp.ClientTimeout(total=AIOHTTP_CLIENT_TIMEOUT),
            )

            r = await session.request(
                method="POST",
                url=request_url,
                data=payload,
                headers=headers,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
            )

            # Check if response is SSE
            if "text/event-stream" in r.headers.get("Content-Type", ""):
                streaming = True
                return StreamingResponse(
                    r.content,
                    status_code=r.status,
                    headers=dict(r.headers),
                    background=BackgroundTask(
                        cleanup_response, response=r, session=session
                    ),
                )
            else:
                try:
                    response = await r.json()
                except Exception as e:
                    log.error(e)
                    response = await r.text()

                r.raise_for_status()
                return response
        except Exception as e:
            log.exception(e)

            detail = None
            if isinstance(response, dict):
                if "error" in response:
                    detail = f"{response['error']['message'] if 'message' in response['error'] else response['error']}"
            elif isinstance(response, str):
                detail = response

            raise HTTPException(
                status_code=r.status if r else 500,
                detail=detail if detail else "Open WebUI: Server Connection Error",
            )
        finally:
            if not streaming and session:
                if r:
                    r.close()
                await session.close()

    async def obtain_user_api_key(self, oidc_user_id: str) -> str:
        req = {
            "oidc_user_id": oidc_user_id,
            "token_name": self.valves.TOKEN_NAME,
            "token_group": self.valves.TOKEN_GROUP,
        }

        async with aiohttp.ClientSession(
            trust_env=True, timeout=aiohttp.ClientTimeout(total=AIOHTTP_CLIENT_TIMEOUT)
        ) as session:
            async with session.post(url=self.valves.API_TOKEN_URL, json=req) as resp:
                if resp.headers.get("content-type") == "application/json":
                    j = await resp.json()
                    return resp.status, j.get("token", None)
                else:
                    return resp.status, await resp.text()
