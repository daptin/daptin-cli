package cmd

import (
	"log/slog"
	"os"
)

// InitLogger configures the default slog logger.
// debug=true sets LevelDebug; otherwise LevelWarn (only warnings/errors show).
func InitLogger(debug bool) {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

// SetLogLevel changes the log level on the default logger by replacing the handler.
func SetLogLevel(level slog.Level) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
