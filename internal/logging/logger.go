package logging

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
)

// Init sets the global logger to JSON output for CloudWatch
func Init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}

// WithRequestContext returns a logger enriched with request-scoped fields
func WithRequestContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		logger = logger.With("request_id", reqID)
	}
	return logger
}
