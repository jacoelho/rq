package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

type Limiter struct {
	limiter *rate.Limiter
}

// New uses 0 or negative limit for no rate limiting.
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

func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow is non-blocking and useful for checking throttling.
func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}

// Reserve is for advanced rate limiting scenarios.
func (l *Limiter) Reserve() *rate.Reservation {
	return l.limiter.Reserve()
}

// SetLimit can be called at runtime.
func (l *Limiter) SetLimit(requestsPerSecond float64) {
	if requestsPerSecond <= 0 {
		l.limiter.SetLimit(rate.Inf)
	} else {
		l.limiter.SetLimit(rate.Limit(requestsPerSecond))
	}
}

func (l *Limiter) Limit() float64 {
	limit := l.limiter.Limit()
	if limit == rate.Inf {
		return 0 // Indicate no rate limiting
	}
	return float64(limit)
}
