package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerSecond float64
		expectUnlimited   bool
	}{
		{
			name:              "unlimited_zero",
			requestsPerSecond: 0,
			expectUnlimited:   true,
		},
		{
			name:              "unlimited_negative",
			requestsPerSecond: -1,
			expectUnlimited:   true,
		},
		{
			name:              "limited_one_per_second",
			requestsPerSecond: 1,
			expectUnlimited:   false,
		},
		{
			name:              "limited_ten_per_second",
			requestsPerSecond: 10,
			expectUnlimited:   false,
		},
		{
			name:              "limited_fractional",
			requestsPerSecond: 0.5,
			expectUnlimited:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := New(tt.requestsPerSecond)
			if limiter == nil {
				t.Fatal("New() returned nil")
			}

			limit := limiter.Limit()
			if tt.expectUnlimited {
				if limit != 0 {
					t.Errorf("Expected unlimited (0), got %f", limit)
				}
			} else {
				if limit != tt.requestsPerSecond {
					t.Errorf("Expected limit %f, got %f", tt.requestsPerSecond, limit)
				}
			}
		})
	}
}

func TestLimiter_Allow(t *testing.T) {
	t.Run("unlimited_allows_all", func(t *testing.T) {
		limiter := New(0) // Unlimited

		// Should allow multiple requests immediately
		for i := range 10 {
			if !limiter.Allow() {
				t.Errorf("Unlimited limiter should allow request %d", i)
			}
		}
	})

	t.Run("limited_respects_rate", func(t *testing.T) {
		limiter := New(1) // 1 request per second

		// First request should be allowed
		if !limiter.Allow() {
			t.Error("First request should be allowed")
		}

		// Second immediate request should be denied
		if limiter.Allow() {
			t.Error("Second immediate request should be denied")
		}
	})
}

func TestLimiter_Wait(t *testing.T) {
	t.Run("unlimited_no_wait", func(t *testing.T) {
		limiter := New(0) // Unlimited
		ctx := context.Background()

		start := time.Now()
		if err := limiter.Wait(ctx); err != nil {
			t.Errorf("Wait() failed: %v", err)
		}
		duration := time.Since(start)

		// Should complete almost immediately
		if duration > 10*time.Millisecond {
			t.Errorf("Unlimited limiter took too long: %v", duration)
		}
	})

	t.Run("limited_waits_appropriately", func(t *testing.T) {
		limiter := New(10) // 10 requests per second = 100ms between requests
		ctx := context.Background()

		// First request should be immediate
		start := time.Now()
		if err := limiter.Wait(ctx); err != nil {
			t.Errorf("First Wait() failed: %v", err)
		}
		firstDuration := time.Since(start)

		if firstDuration > 10*time.Millisecond {
			t.Errorf("First request took too long: %v", firstDuration)
		}

		// Second request should wait
		start = time.Now()
		if err := limiter.Wait(ctx); err != nil {
			t.Errorf("Second Wait() failed: %v", err)
		}
		secondDuration := time.Since(start)

		// Should wait approximately 100ms (allow some tolerance)
		if secondDuration < 80*time.Millisecond || secondDuration > 120*time.Millisecond {
			t.Errorf("Second request wait time unexpected: %v (expected ~100ms)", secondDuration)
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		limiter := New(1) // 1 request per second
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Use up the first allowed request
		if err := limiter.Wait(context.Background()); err != nil {
			t.Errorf("First Wait() failed: %v", err)
		}

		// Second request should be cancelled by context timeout
		if err := limiter.Wait(ctx); err == nil {
			t.Error("Expected context cancellation error")
		} else {
			// The golang.org/x/time/rate package returns a custom error type
			// Just check that we got an error, which indicates context cancellation worked
			t.Logf("Got expected context cancellation error: %v", err)
		}
	})
}

func TestLimiter_SetLimit(t *testing.T) {
	limiter := New(1) // Start with 1 request per second

	// Verify initial limit
	if limit := limiter.Limit(); limit != 1 {
		t.Errorf("Initial limit should be 1, got %f", limit)
	}

	// Change to unlimited
	limiter.SetLimit(0)
	if limit := limiter.Limit(); limit != 0 {
		t.Errorf("After SetLimit(0), limit should be 0, got %f", limit)
	}

	// Change to 5 requests per second
	limiter.SetLimit(5)
	if limit := limiter.Limit(); limit != 5 {
		t.Errorf("After SetLimit(5), limit should be 5, got %f", limit)
	}

	// Change to negative (should become unlimited)
	limiter.SetLimit(-1)
	if limit := limiter.Limit(); limit != 0 {
		t.Errorf("After SetLimit(-1), limit should be 0 (unlimited), got %f", limit)
	}
}

func TestLimiter_Reserve(t *testing.T) {
	limiter := New(2) // 2 requests per second

	// First reservation should be immediate
	r1 := limiter.Reserve()
	if r1.Delay() > time.Millisecond {
		t.Errorf("First reservation should be immediate, got delay %v", r1.Delay())
	}

	// Second reservation should have some delay
	r2 := limiter.Reserve()
	if r2.Delay() < 400*time.Millisecond || r2.Delay() > 600*time.Millisecond {
		t.Errorf("Second reservation delay should be ~500ms, got %v", r2.Delay())
	}

	// Cancel the reservations to clean up
	r1.Cancel()
	r2.Cancel()
}

func TestLimiter_Integration(t *testing.T) {
	// Test a realistic scenario with multiple requests
	limiter := New(5) // 5 requests per second
	ctx := context.Background()

	start := time.Now()

	// Make 3 requests
	for i := range 3 {
		if err := limiter.Wait(ctx); err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	duration := time.Since(start)

	// First request immediate, second waits 200ms, third waits 200ms more
	// Total should be around 400ms
	expectedDuration := 400 * time.Millisecond
	tolerance := 50 * time.Millisecond

	if duration < expectedDuration-tolerance || duration > expectedDuration+tolerance {
		t.Errorf("Total duration %v not within expected range %v ± %v",
			duration, expectedDuration, tolerance)
	}
}
