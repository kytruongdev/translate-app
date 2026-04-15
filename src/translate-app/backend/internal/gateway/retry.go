package gateway

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/sashabaranov/go-openai"
)

const (
	retryMaxAttempts = 3
	retryInitialWait = time.Second
	retryMultiplier  = 2.0
)

// sleepBackoff waits with exponential backoff: ~1s, ~2s, ~4s and ±20% jitter before the next attempt.
// attempt is zero-based: first retry delay uses attempt 0.
func sleepBackoff(ctx context.Context, attempt int) error {
	if attempt < 0 {
		attempt = 0
	}
	baseSecs := float64(retryInitialWait/time.Second) * powFloat(retryMultiplier, attempt)
	base := time.Duration(baseSecs * float64(time.Second))
	jitter := time.Duration(float64(base) * 0.2 * (2*rand.Float64() - 1))
	d := base + jitter
	if d < 50*time.Millisecond {
		d = 50 * time.Millisecond
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func powFloat(mul float64, exp int) float64 {
	out := 1.0
	for i := 0; i < exp; i++ {
		out *= mul
	}
	return out
}

// IsConnectionRefused reports whether err is a local connection refused.
func IsConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	var op *net.OpError
	if errors.As(err, &op) {
		if errors.Is(op.Err, syscall.ECONNREFUSED) {
			return true
		}
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused")
}

// IsRetryableOpenAI returns true for 429, 500, 502, 503 (OpenAI-compatible API).
func IsRetryableOpenAI(err error) bool {
	if err == nil || IsConnectionRefused(err) {
		return false
	}
	var api *openai.APIError
	if errors.As(err, &api) {
		switch api.HTTPStatusCode {
		case 429, 500, 502, 503:
			return true
		default:
			return false
		}
	}
	var re *openai.RequestError
	if errors.As(err, &re) {
		switch re.HTTPStatusCode {
		case 429, 500, 502, 503:
			return true
		default:
			return false
		}
	}
	return false
}
