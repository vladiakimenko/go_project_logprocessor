package bootstrap

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func getLogger(settings *config) (*slog.Logger, error) {
	var level slog.Level
	err := level.UnmarshalText([]byte(settings.Logging.Level))
	if err != nil {
		return nil, err
	}
	var handler slog.Handler
	if Settings.Logging.Json {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler), nil
}
