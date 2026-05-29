package task

import (
	"math"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Exponential backoff
// ─────────────────────────────────────────────────────────────────────────────

// BackoffStrategy computes the delay before the next retry attempt.
// It uses truncated exponential backoff:
//
//	delay = base * multiplier^(attempt-1)
//
// capped at maxDelay to prevent unbounded waits.
type BackoffStrategy struct {
	Base       time.Duration // delay after first failure
	Multiplier float64       // growth factor per attempt
	MaxDelay   time.Duration // upper bound
}

// DefaultBackoff returns a sensible default: 5s, 25s, 125s (capped at 10m).
func DefaultBackoff(base time.Duration) BackoffStrategy {
	return BackoffStrategy{
		Base:       base,
		Multiplier: 5.0,
		MaxDelay:   10 * time.Minute,
	}
}

// Delay returns the wait duration for the given attempt number (1-based).
func (b BackoffStrategy) Delay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	d := float64(b.Base) * math.Pow(b.Multiplier, float64(attempt-1))
	if d > float64(b.MaxDelay) {
		return b.MaxDelay
	}
	return time.Duration(d)
}
