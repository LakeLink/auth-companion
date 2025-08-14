package main

import (
	"github.com/lakelink/auth-companion/in"
	"github.com/lakelink/auth-companion/misc"
	"github.com/lakelink/auth-companion/out"
	"github.com/spf13/viper"
)

func main() {
	misc.SetupConfig()
	misc.SetupLogger()

	newApiActor := out.NewNewApiActor(viper.GetString("newapi.db_path"))
	feishuActor := out.NewFeishuActor()
	zitadelActor := out.NewZitadelActor(viper.GetString("zitadel.domain"), viper.GetString("zitadel.pat"), viper.GetString("zitadel.feishu_idp_id"))
	done := make(chan error)
	go in.StartEchoListener(newApiActor, feishuActor, done)
	go in.StartFeishuListener(zitadelActor, done)
	<-done
}
