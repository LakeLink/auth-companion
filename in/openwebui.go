package in

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/lakelink/auth-companion/out"
)

type OpenWebUiEnsureTokenRequest struct {
	OidcUserId string `json:"oidc_user_id"`
	TokenName  string `json:"token_name"`
	TokenGroup string `json:"token_group"`
}

type OpenWebUiHandler struct {
	newApiActor *out.NewApiActor
}

func SetupOpenWebUiEndpoints(g *echo.Group, newApiActor *out.NewApiActor) {
	h := OpenWebUiHandler{newApiActor}
	g.POST("/ensure_token", h.handleEnsureToken)
}

func (h *OpenWebUiHandler) handleEnsureToken(c echo.Context) error {
	var req OpenWebUiEnsureTokenRequest
	err := c.Bind(&req)
	if err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}

	resp, err := h.newApiActor.EnsureToken(req.OidcUserId, req.TokenName, req.TokenGroup)
	if errors.Is(err, sql.ErrNoRows) {
		return c.String(http.StatusNotFound, "user not found")
	} else if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	} else {
		return c.JSON(http.StatusOK, resp)
	}
}
