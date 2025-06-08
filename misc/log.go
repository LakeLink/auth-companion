package misc

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func SetupLogger() error {
	toConsole := viper.GetBool("log.console")
	toFile := viper.GetString("log.path")

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	var createFileErr error = nil
	var logFile *os.File
	if len(toFile) > 0 {
		logFile, createFileErr = os.OpenFile(
			toFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0664,
		)
		if createFileErr != nil {
			toConsole = true
		}
	}

	if toConsole {
		// Multi-writer setup

		multi := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, logFile)
		log.Logger = zerolog.New(multi).With().Timestamp().Caller().Logger()
	} else {
		log.Logger = zerolog.New(logFile).With().Timestamp().Caller().Logger()
	}

	if createFileErr != nil {
		log.Error().Err(createFileErr).Str("path", toFile).Msg("log file cannot be created")
	}

	return createFileErr
}