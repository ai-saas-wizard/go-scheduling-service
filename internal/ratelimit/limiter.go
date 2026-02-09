package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var (
	openaiLimiter *rate.Limiter
	once          sync.Once
)

const (
	OpenAIRequestsPerMinute = 10
	OpenAIBurstSize         = 3
)

// GetOpenAILimiter returns the singleton rate limiter
func GetOpenAILimiter() *rate.Limiter {
	once.Do(func() {
		openaiLimiter = rate.NewLimiter(rate.Every(time.Minute/OpenAIRequestsPerMinute), OpenAIBurstSize)
	})
	return openaiLimiter
}

// WaitForOpenAI blocks until the rate limiter allows a request
func WaitForOpenAI(ctx context.Context) error {
	limiter := GetOpenAILimiter()
	if err := limiter.Wait(ctx); err != nil {
		slog.WarnContext(ctx, "openai_rate_limited", "error", err)
		return fmt.Errorf("rate limited: %w", err)
	}
	return nil
}
