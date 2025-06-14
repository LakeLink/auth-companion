package out

import (
	"database/sql"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = keyChars[rand.Intn(len(keyChars))]
	}
	return string(b)
}

type NewApiActor struct {
	db *sql.DB
}

type NewApiEnsureTokenResponse struct {
	TokenId int    `json:"token_id"`
	Token   string `json:"token"`
}

func NewNewApiActor(dbPath string) *NewApiActor {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}

	h := &NewApiActor{
		db: db,
	}

	return h
}

func (h *NewApiActor) EnsureToken(oidcUserId, tokenName, tokenGroup string) (*NewApiEnsureTokenResponse, error) {
	row := h.db.QueryRow("SELECT id, username, oidc_id FROM users WHERE oidc_id = ? AND deleted_at IS NULL", oidcUserId)

	var user_id int
	var username, oidc_id string
	if err := row.Scan(&user_id, &username, &oidc_id); err != nil {
		return nil, err
	}

	log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("user found")

	now := time.Now().Unix()

	res, err := h.db.Exec(
		`INSERT INTO tokens(user_id, key, name, created_time, accessed_time, unlimited_quota, [group])
		SELECT ?, ?, ?, ?, ?, 1, ?
		WHERE NOT EXISTS (SELECT id FROM tokens WHERE user_id = ? AND name = ? AND deleted_at IS NULL)`,
		user_id, RandStringBytes(48), tokenName, now, now, tokenGroup, user_id, tokenName,
	)
	if err != nil {
		return nil, err
	}

	if n, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if n == 1 {
		log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("token created")
	} else {
		log.Info().Int("user_id", user_id).Str("username", username).Str("oidc_id", oidc_id).Msg("token already exists")
	}

	var token_id int
	var token string
	row = h.db.QueryRow("SELECT id, key FROM tokens WHERE user_id = ? AND name = ? AND deleted_at IS NULL", user_id, tokenName)
	if err := row.Scan(&token_id, &token); err != nil {
		return nil, err
	}

	token = "sk-" + token

	return &NewApiEnsureTokenResponse{token_id, token}, nil
}
