package misc

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type NewApiWebhookConfig struct {
	Src   string
	Actor string
	Dst   string
}

func SetupConfig() {
	// Set the file name and path (without extension)
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".") // Look for the configuration file in current directory.

	// Set defaults optionally
	viper.SetDefault("listen_addr", ":1323")

	viper.SetDefault("log.console", true)
	viper.SetDefault("log.path", "auth_companion.log")

	viper.SetDefault("newapi.db_path", "one-api.db")
	viper.SetDefault("newapi.webhooks", []NewApiWebhookConfig{
		{
			"default", "feishu", "open_id:ou_7d8a6e6df7621556ce0d21922b676706ccs",
		},
	})

	viper.SetDefault("feishu.app_id", "")
	viper.SetDefault("feishu.app_secret", "")
	viper.SetDefault("feishu.verification_token", "")
	viper.SetDefault("feishu.encrypt_key", "")

	viper.SetDefault("zitadel.domain", "")
	viper.SetDefault("zitadel.pat", "")
	viper.SetDefault("zitadel.feishu_idp_id", "")

	// Check if config file exists
	configFile := "config.toml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config file not found; create with defaults
		fmt.Println("Config file not found, creating default config...")

		// Save default configuration to file
		if err := viper.SafeWriteConfigAs(configFile); err != nil {
			fmt.Printf("ERROR: couldn't write default config file: %v", err)
			panic(err)
		}
	}

	// Read in config file and handle any errors
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("ERROR: couldn't read config file: %v", err)
		panic(err)
	}
}
