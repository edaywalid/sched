// Package observability owns logging, metrics, and tracing setup for
// the engine, workers, and dashboard binaries.
package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects between human-readable text and machine-readable JSON
// output. Configured by the SCHED_LOG_FORMAT env var ("json" or "text").
type Format int

const (
	FormatText Format = iota
	FormatJSON
)

// NewLogger constructs a slog.Logger and installs it as the package
// default so any log/slog call automatically picks it up.
//
// component is added as a static attribute on every log line so a
// shared log stream can be filtered by binary.
func NewLogger(component string) *slog.Logger {
	return newLogger(component, os.Stderr, formatFromEnv(), levelFromEnv())
}

func newLogger(component string, w io.Writer, format Format, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch format {
	case FormatJSON:
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}
	if component != "" {
		handler = handler.WithAttrs([]slog.Attr{slog.String("component", component)})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func formatFromEnv() Format {
	switch strings.ToLower(os.Getenv("SCHED_LOG_FORMAT")) {
	case "json":
		return FormatJSON
	default:
		return FormatText
	}
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv("SCHED_LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
