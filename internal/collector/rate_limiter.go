package collector

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter manages GitHub API rate limiting
type RateLimiter interface {
	Wait(ctx context.Context) error
	CheckLimit() (remaining int, resetTime time.Time, err error)
	UpdateLimit(remaining int, resetTime time.Time)
}

// githubRateLimiter implements RateLimiter for GitHub API
type githubRateLimiter struct {
	mu        sync.Mutex
	remaining int
	resetTime time.Time
	minDelay  time.Duration
	lastCall  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() RateLimiter {
	return &githubRateLimiter{
		remaining: 5000, // GitHub API default limit
		resetTime: time.Now().Add(time.Hour),
		minDelay:  100 * time.Millisecond, // Minimum delay between requests
	}
}

// Wait waits until it's safe to make another API call
func (r *githubRateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we need to wait for rate limit reset
	if r.remaining <= 10 {
		waitDuration := time.Until(r.resetTime)
		if waitDuration > 0 {
			fmt.Printf("  Rate limit low (%d remaining), waiting %v until reset...\n", r.remaining, waitDuration.Round(time.Second))
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				r.mu.Lock()
				return ctx.Err()
			case <-time.After(waitDuration):
				r.mu.Lock()
			}
			fmt.Printf("  Rate limit reset, continuing...\n")
		}
		// Reset after waiting
		r.remaining = 5000
		r.resetTime = time.Now().Add(time.Hour)
	}

	// Ensure minimum delay between requests
	elapsed := time.Since(r.lastCall)
	if elapsed < r.minDelay {
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			r.mu.Lock()
			return ctx.Err()
		case <-time.After(r.minDelay - elapsed):
			r.mu.Lock()
		}
	}

	r.lastCall = time.Now()
	return nil
}

// CheckLimit returns the current rate limit status
func (r *githubRateLimiter) CheckLimit() (remaining int, resetTime time.Time, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.remaining, r.resetTime, nil
}

// UpdateLimit updates the rate limit from API response headers
func (r *githubRateLimiter) UpdateLimit(remaining int, resetTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remaining = remaining
	r.resetTime = resetTime
}
