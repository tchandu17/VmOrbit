package proxmox

import (
	"context"
	"math"
	"strings"
	"time"
)

// retryConfig controls the exponential-backoff retry strategy.
type retryConfig struct {
	// maxAttempts is the total number of attempts (including the first).
	// Zero or negative means try once (no retries).
	maxAttempts int
	// baseDelay is the wait before the second attempt.
	baseDelay time.Duration
	// maxDelay caps the computed backoff.
	maxDelay time.Duration
	// shouldRetry is called with the error from each failed attempt.
	// Return true to retry, false to abort immediately.
	// If nil, all errors are retried.
	shouldRetry func(err error) bool
}

// withRetry executes fn up to cfg.maxAttempts times, applying exponential
// backoff between attempts. The attempt counter passed to fn is 1-based.
//
// The retry loop respects ctx cancellation — if the context is cancelled
// during a sleep the function returns ctx.Err() immediately.
func withRetry(ctx context.Context, cfg retryConfig, fn func(attempt int) error) error {
	if cfg.maxAttempts <= 0 {
		cfg.maxAttempts = 1
	}
	if cfg.baseDelay == 0 {
		cfg.baseDelay = time.Second
	}
	if cfg.maxDelay == 0 {
		cfg.maxDelay = 30 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}

		// Check context before deciding to retry.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Last attempt — don't sleep, just return the error.
		if attempt == cfg.maxAttempts {
			break
		}

		// Check if this error class is retryable.
		if cfg.shouldRetry != nil && !cfg.shouldRetry(lastErr) {
			return lastErr
		}

		delay := backoffDelay(cfg.baseDelay, cfg.maxDelay, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// backoffDelay computes the delay for attempt n using exponential backoff:
// delay = min(maxDelay, baseDelay * 2^(n-1)).
func backoffDelay(base, max time.Duration, attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	d := time.Duration(float64(base) * exp)
	if d > max {
		d = max
	}
	return d
}

// isRetryableConnectError returns true for transient connection errors that
// are worth retrying (network timeouts, temporary DNS failures, etc.).
// Authentication failures and invalid-URL errors are not retried.
func isRetryableConnectError(err error) bool {
	if err == nil {
		return false
	}
	if isProxmoxAuthError(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	transient := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"no such host",
		"eof",
		"broken pipe",
		"i/o timeout",
		"network is unreachable",
		"tls handshake",
	}
	for _, t := range transient {
		if strings.Contains(msg, t) {
			return true
		}
	}
	return false
}

// isProxmoxAuthError returns true if the error looks like a Proxmox
// authentication or authorisation failure.
func isProxmoxAuthError(err error) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*apiError); ok {
		return ae.isAuthError()
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"401", "403", "unauthorized", "forbidden", "permission denied", "authentication"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// isRetryableOperationError returns true for transient Proxmox operation
// errors that are safe to retry (resource busy, task conflicts, etc.).
func isRetryableOperationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	retryable := []string{
		"resource in use",
		"task in progress",
		"locked",
		"busy",
		"timeout",
		"eof",
		"connection reset",
	}
	for _, r := range retryable {
		if strings.Contains(msg, r) {
			return true
		}
	}
	return false
}

// withOperationRetry is a convenience wrapper for retrying VM operations
// with the standard operation retry policy (3 attempts, 1 s base delay).
func withOperationRetry(ctx context.Context, fn func(attempt int) error) error {
	return withRetry(ctx, retryConfig{
		maxAttempts: 3,
		baseDelay:   1 * time.Second,
		maxDelay:    10 * time.Second,
		shouldRetry: isRetryableOperationError,
	}, fn)
}
