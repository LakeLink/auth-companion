"""
title: LakeLink API Key Configurator
author: yiffyi
version: 0.1
required_open_webui_version: 0.5.3
"""

import json
import asyncio
import aiohttp

from typing import List, Union, Generator, Iterator
from pydantic import BaseModel, Field
from typing import Optional
from fastapi import Request

from open_webui.models.users import Users, UserSettings

from open_webui.env import AIOHTTP_CLIENT_TIMEOUT


class Tools:
    class Valves(BaseModel):
        api_base_url: str = Field(default="https://api.openai.com/v1")
        api_token_url: str = Field(default="https://localhost/newapi/ensure_token")
        token_name: str = Field(default="open-webui")
        token_group: str = Field(default="open-webui")
        pass

    def __init__(self):
        self.valves = self.Valves()
        pass

    async def enable_model_access(
        self,
        __request__: Request,
        __user__: dict,
        __event_emitter__=None,
    ) -> str:
        await __event_emitter__(
            {
                "type": "status",
                "data": {"description": "Enabling model access", "done": False},
            }
        )

        try:
            user = Users.get_user_by_id(__user__["id"])
            # print(__user__, __request__, sep="\n")
            # print(user.oauth_sub.startswith("oidc@"), user.oauth_sub)

            return await self.__execute_enable_model_access__(user, __event_emitter__)

        except Exception as e:
            await __event_emitter__(
                {
                    "type": "status",
                    "data": {"description": f"An error occured: {e}", "done": True},
                }
            )

            return f"Tell the user: {e}"

        finally:
            pass
            # images = await image_generations(
            #     request=__request__,
            #     form_data=GenerateImageForm(**{"prompt": prompt}),
            #     user=Users.get_user_by_id(__user__["id"]),
            # )

            # for image in images:
            #     await __event_emitter__(
            #         {
            #             "type": "message",
            #             "data": {"content": f"![Generated Image]({image['url']})"},
            #         }
            #     )

            # return f"Notify the user that the image has been successfully generated"

    async def __execute_enable_model_access__(self, user, __event_emitter__):
        if not user.oauth_sub.startswith("oidc@"):
            return await self.__emit_error__(
                "You are not registered through LakeLink ZITADEL.",
                emitter=__event_emitter__,
            )


        await __event_emitter__(
            {
                "type": "status",
                "data": {"description": "Obtaining API key", "done": False},
            }
        )
        oidc_user_id = user.oauth_sub.split("@")[1]
        status, token = await self.__obtain_user_api_token__(oidc_user_id)
        if status == 404:
            return await self.__emit_error__(
                "User not found at LakeLink AI Aggregator.",
                notification="Please first login using OIDC at https://ai.lklk.tech.",
            )
        elif status != 200:
            return await self.__emit_error__(
                f"An error occured while obtaining API key: {token}",
                emitter=__event_emitter__,
            )


        await __event_emitter__(
            {
                "type": "status",
                "data": {"description": "Configuring direct connection", "done": False},
            }
        )
        try:
            self.__update_user_direct_connection__(
                user.id, user.settings and user.settings.ui, token
            )
        except Exception as e:
            raise
            # return self.__emit_error__(
            #     "Could not update your settings in database.",
            #     emitter=__event_emitter__,
            # )

        await __event_emitter__(
            {
                "type": "notification",
                "data": {"type": "success", "content": "Operation completed."},
            }
        )

        await __event_emitter__(
            {
                "type": "status",
                "data": {"description": "API Key configured", "done": True},
            }
        )

        return "✅ Please **refresh the page** to access newly enabled models.\nRemember to top-up credit balance at https://ai.lklk.tech."

    async def __obtain_user_api_token__(self, oidc_user_id: str) -> str:
        req = {
            "oidc_user_id": oidc_user_id,
            "token_name": self.valves.token_name,
            "token_group": self.valves.token_group,
        }

        async with aiohttp.ClientSession(
            trust_env=True, timeout=aiohttp.ClientTimeout(total=AIOHTTP_CLIENT_TIMEOUT)
        ) as session:
            async with session.post(url=self.valves.api_token_url, json=req) as resp:
                if resp.headers.get('content-type') == 'application/json':
                    j = await resp.json()
                    return resp.status, j.get("token", None)
                else:
                    return resp.status, await resp.text()

    def __update_user_direct_connection__(
        self, user_id: str, prev_ui_settings: dict, token: str
    ):
        cfg = {
            "directConnections": {
                "OPENAI_API_BASE_URLS": [self.valves.api_base_url],
                "OPENAI_API_KEYS": [token],
                "OPENAI_API_CONFIGS": {
                    "0": {
                        "enable": True,
                        "tags": [],
                        "prefix_id": "",
                        "model_ids": [],
                        "connection_type": "external",
                    }
                },
            },
        }

        if prev_ui_settings:
            prev_ui_settings.update(cfg)
            ui = prev_ui_settings
        else:
            ui = cfg
        return Users.update_user_settings_by_id(
            user_id,
            {"ui": ui},
        )

    async def __emit_error__(
        self,
        msg: str,
        notification: str = "Please contact administrator for help.",
        emitter=None,
    ) -> str:
        await emitter(
            {
                "type": "notification",
                "data": {
                    "type": "warning",
                    "content": notification,
                },
            }
        )
        return f"❗ **{msg}**\n{notification}"
