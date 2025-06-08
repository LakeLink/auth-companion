package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lakelink/auth-companion/api"
	"github.com/lakelink/auth-companion/misc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func main() {
	misc.SetupConfig()
	misc.SetupLogger()

	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:       true,
		LogStatus:    true,
		LogError:     true,
		LogRemoteIP:  true,
		LogHost:      true,
		LogMethod:    true,
		LogUserAgent: true,
		HandleError:  true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			evt := log.Info()
			if v.Error != nil {
				evt = log.Error()
			}

			evt.Str("uri", v.URI).
				Str("remote_ip", v.RemoteIP).
				Str("host", v.Host).
				Str("user_agent", v.UserAgent).
				Str("method", v.Method).
				Int("status", v.Status).
				Msg("request")
			return nil
		},
	}))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	gFeishu := e.Group("/feishu")
	api.SetupFeishuEndpoints(gFeishu)

	e.Logger.Fatal(e.Start(viper.GetString("listen_addr")))
}
