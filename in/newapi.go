package in

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/lakelink/auth-companion/misc"
	"github.com/lakelink/auth-companion/out"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type newApiWebhookPayload struct {
	Type      string        `json:"type"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Values    []interface{} `json:"values,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

type NewApiEventHandler struct {
	feishuActor *out.FeishuActor
	dst         map[string]string
}

func (h *NewApiEventHandler) handleNotification(c echo.Context) error {
	src := c.Param("source")

	if dst, ok := h.dst[src]; ok {

		receiver := strings.SplitN(dst, ":", 2)

		if len(receiver) < 2 {
			log.Error().Strs("receiver", receiver).Msg("incorrect dst, missing receive_id_type or receive_id")
			return echo.NewHTTPError(http.StatusInternalServerError, "incorrect dst format")
		}

		var body newApiWebhookPayload
		if err := c.Bind(&body); err != nil {
			return err
		}

		log.Info().
			Str("src", src).
			Str("dst", dst).
			Str("type", body.Type).
			Str("title", body.Title).
			Str("content", body.Content).
			Any("values", body.Values).
			Int64("timestamp", body.Timestamp).
			Msg("received new webhook event")

		content := body.Content
		if len(body.Values) > 0 {
			content = fmt.Sprintf(strings.ReplaceAll(body.Content, "{{value}}", "%+v"), body.Values...)
		}

		err := h.feishuActor.SendTextMessage(
			receiver[0],
			receiver[1],
			fmt.Sprintf(
				"Received: from %s\nFrom: %s\nSubject: %s\n\n%s",
				c.RealIP(), src, body.Title, content,
			),
		)
		return err
	} else {
		return echo.NewHTTPError(http.StatusNotFound, "src -> dst mapping not configured")
	}

}

func SetupNewApiEndpoints(g *echo.Group, feishu *out.FeishuActor) {
	m := map[string]string{}

	var mappings []misc.NewApiWebhookConfig
	viper.UnmarshalKey("newapi.webhooks", &mappings)
	for _, v := range mappings {
		if v.Actor != "feishu" {
			log.Error().Str("src", v.Src).Str("dst", v.Dst).Str("actor", v.Actor).Msg("webhook handler only supports FeishuActor")
			continue
		}
		m[v.Src] = v.Dst
	}
	h := NewApiEventHandler{feishu, m}

	g.POST("/notification/:source", h.handleNotification)
}
