package observability

import (
	"io"
	"log/slog"
	"os"
)

// Logger is the package-level structured logger. Initialised to a discard handler so
// any call before InitLogger is a no-op rather than a nil-pointer panic.
var Logger = slog.New(slog.NewJSONHandler(io.Discard, nil))

func InitLogger(level slog.Level) {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(os.Stderr, opts)
	Logger = slog.New(handler)
}
