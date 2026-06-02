package observability

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func InitLogger(level slog.Level) {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(os.Stderr, opts)
	Logger = slog.New(handler)
}
