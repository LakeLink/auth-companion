package api

import (
	"database/sql"
	"math/rand"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = keyChars[rand.Intn(len(keyChars))]
	}
	return string(b)
}

type NewApiHandler struct {
	db *sql.DB
}

type NewApiEnsureTokenRequest struct {
	OidcUserId string `json:"oidc_user_id"`
	TokenName  string `json:"token_name"`
	TokenGroup string `json:"token_group"`
}

type NewApiEnsureTokenResponse struct {
	TokenId int    `json:"token_id"`
	Token   string `json:"token"`
}

func SetupNewApiEndpoints(g *echo.Group) {
	db, err := sql.Open("sqlite3", viper.GetString("newapi.db_path"))
	if err != nil {
		panic(err)
	}

	h := &NewApiHandler{
		db: db,
	}
	g.POST("/ensure_token", h.ensureToken)
}

func (h *NewApiHandler) ensureToken(c echo.Context) error {
	var req NewApiEnsureTokenRequest
	err := c.Bind(&req)
	if err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}

	row := h.db.QueryRow("SELECT id, username, oidc_id FROM users WHERE oidc_id = ? AND deleted_at IS NULL", req.OidcUserId)

	var user_id int
	var username, oidc_id string
	if err := row.Scan(&user_id, &username, &oidc_id); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("user found")

	now := time.Now().Unix()

	res, err := h.db.Exec(
		`INSERT INTO tokens(user_id, key, name, created_time, accessed_time, unlimited_quota, [group])
		SELECT ?, ?, ?, ?, ?, 1, ?
		WHERE NOT EXISTS (SELECT id FROM tokens WHERE user_id = ? AND name = ? AND deleted_at IS NULL)`,
		user_id, RandStringBytes(48), req.TokenName, now, now, req.TokenGroup, user_id, req.TokenName,
	)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	if n, err := res.RowsAffected(); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	} else if n == 1 {
		log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("token created")
	} else {
		log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("token already exists")
	}

	var token_id int
	var token string
	row = h.db.QueryRow("SELECT id, key FROM tokens WHERE user_id = ? AND name = ? AND deleted_at IS NULL", user_id, req.TokenName)
	if err := row.Scan(&token_id, &token); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	token = "sk-"+token;

	return c.JSON(http.StatusOK, NewApiEnsureTokenResponse{token_id, token})
}
