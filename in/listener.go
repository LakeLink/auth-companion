package in

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lakelink/auth-companion/out"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func StartFeishuListener(zitadelActor *out.ZitadelActor, done chan<- error) {
	eventHandler := dispatcher.NewEventDispatcher(viper.GetString("feishu.verification_token"), viper.GetString("feishu.encrypt_key"))
	eventHandler = SetupFeishuEventHandler(eventHandler, zitadelActor)

	app_id, app_secret := viper.GetString("feishu.app_id"), viper.GetString("feishu.app_secret")
	cli := larkws.NewClient(app_id, app_secret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)

	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}
	done <- err
}

func StartEchoListener(newApiActor *out.NewApiActor, feishuActor *out.FeishuActor, done chan<- error) {

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

	gZitadel := e.Group("/zitadel")
	SetupZitadelEndpoints(gZitadel)

	gOpenWebUi := e.Group("/open-webui")
	SetupOpenWebUiEndpoints(gOpenWebUi, newApiActor)

	gNewApi := e.Group("/newapi")
	SetupNewApiEndpoints(gNewApi, feishuActor)

	err := e.Start(viper.GetString("listen_addr"))
	e.Logger.Fatal(err)
	done <- err
}
