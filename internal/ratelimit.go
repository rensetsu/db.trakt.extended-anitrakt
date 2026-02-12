package internal

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements token bucket algorithm for rate limiting
type RateLimiter struct {
	maxRequests int           // Max requests per window
	windowSize  time.Duration // Time window for rate limit
	tokens      float64       // Current tokens available
	lastRefill  time.Time     // Last time tokens were refilled
	mu          sync.Mutex
}

// NewRateLimiter creates a new rate limiter (1000 requests per 5 minutes for Trakt)
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		maxRequests: 1000,
		windowSize:  5 * time.Minute,
		tokens:      1000,
		lastRefill:  time.Now(),
	}
}

// NewLetterboxdRateLimiter creates a new rate limiter for Letterboxd (100 requests per minute)
func NewLetterboxdRateLimiter() *RateLimiter {
	return &RateLimiter{
		maxRequests: 100,
		windowSize:  1 * time.Minute,
		tokens:      100,
		lastRefill:  time.Now(),
	}
}

// Wait blocks until a token is available, then consumes it
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for {
		now := time.Now()
		elapsed := now.Sub(rl.lastRefill)

		// Refill tokens based on elapsed time
		refillRate := float64(rl.maxRequests) / rl.windowSize.Seconds()
		tokensToAdd := refillRate * elapsed.Seconds()

		if tokensToAdd > 0 {
			rl.tokens = math.Min(float64(rl.maxRequests), rl.tokens+tokensToAdd)
			rl.lastRefill = now
		}

		if rl.tokens >= 1 {
			rl.tokens--
			return
		}

		// Calculate wait time until next token is available
		waitTime := time.Duration((1 - rl.tokens) / refillRate * float64(time.Second))
		if waitTime < 100*time.Millisecond {
			waitTime = 100 * time.Millisecond
		}

		rl.mu.Unlock()
		time.Sleep(waitTime)
		rl.mu.Lock()
	}
}

// RetryConfig contains retry parameters
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retries (default: 3)
	InitialBackoff time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 32s)
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     32 * time.Second,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry on 429 and 403 errors
func RetryWithBackoff(config RetryConfig, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		resp, err := fn()

		// Success case
		if err == nil && resp.StatusCode != 429 && resp.StatusCode != 403 {
			return resp, err
		}

		// Non-retryable error
		if err != nil && (resp == nil || (resp.StatusCode != 429 && resp.StatusCode != 403)) {
			return resp, err
		}

		// 429 or 403 error - should retry
		if resp != nil && (resp.StatusCode == 429 || resp.StatusCode == 403) {
			lastErr = fmt.Errorf("rate limited or blocked (%d)", resp.StatusCode)

			// Check for Retry-After header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if d, err := time.ParseDuration(retryAfter + "s"); err == nil {
					backoff = d
				}
			}

			if attempt < config.MaxRetries {
				resp.Body.Close()
				time.Sleep(backoff)
				backoff = time.Duration(math.Min(
					float64(backoff)*2,
					float64(config.MaxBackoff),
				))
				continue
			}
		}

		return resp, lastErr
	}

	return nil, lastErr
}
