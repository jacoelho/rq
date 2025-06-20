package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter wraps the golang.org/x/time/rate.Limiter with convenience methods
// for HTTP request rate limiting.
type Limiter struct {
	limiter *rate.Limiter
}

// New creates a new rate limiter with the specified requests per second limit.
// A limit of 0 or negative means no rate limiting.
func New(requestsPerSecond float64) *Limiter {
	if requestsPerSecond <= 0 {
		// No rate limiting - use a very high limit
		return &Limiter{
			limiter: rate.NewLimiter(rate.Inf, 1),
		}
	}

	// Allow burst of 1 request, meaning we can make one request immediately
	// but subsequent requests must wait according to the rate limit
	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), 1),
	}
}

// Wait blocks until the rate limiter allows a request to proceed.
// It returns an error if the context is cancelled while waiting.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow returns true if a request is allowed immediately, false otherwise.
// This is non-blocking and useful for checking if a request would be throttled.
func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}

// Reserve returns a reservation that indicates how long the caller must wait
// before the next request is allowed. Use this for more advanced rate limiting scenarios.
func (l *Limiter) Reserve() *rate.Reservation {
	return l.limiter.Reserve()
}

// SetLimit updates the rate limit. This can be called at runtime to adjust
// the rate limiting behavior.
func (l *Limiter) SetLimit(requestsPerSecond float64) {
	if requestsPerSecond <= 0 {
		l.limiter.SetLimit(rate.Inf)
	} else {
		l.limiter.SetLimit(rate.Limit(requestsPerSecond))
	}
}

// Limit returns the current rate limit in requests per second.
func (l *Limiter) Limit() float64 {
	limit := l.limiter.Limit()
	if limit == rate.Inf {
		return 0 // Indicate no rate limiting
	}
	return float64(limit)
}
