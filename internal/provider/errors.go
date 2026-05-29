package provider

import (
	"errors"
	"fmt"
	"net/http"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sentinel errors
// ─────────────────────────────────────────────────────────────────────────────

var (
	// ErrNotConnected is returned when an operation is attempted before Connect.
	ErrNotConnected = errors.New("provider: not connected")

	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("provider: resource not found")

	// ErrUnsupported is returned when a provider does not support an operation.
	// Callers should check Capabilities() before calling optional methods.
	ErrUnsupported = errors.New("provider: operation not supported")

	// ErrAuthFailed is returned when credentials are rejected.
	ErrAuthFailed = errors.New("provider: authentication failed")

	// ErrTimeout is returned when a provider call exceeds its deadline.
	ErrTimeout = errors.New("provider: operation timed out")

	// ErrInvalidState is returned when the VM is in a state that prevents
	// the requested operation (e.g. powering on an already-running VM).
	ErrInvalidState = errors.New("provider: invalid vm state for operation")

	// ErrQuotaExceeded is returned when a resource quota would be breached.
	ErrQuotaExceeded = errors.New("provider: resource quota exceeded")

	// ErrConflict is returned when a resource with the same identity already exists.
	ErrConflict = errors.New("provider: resource already exists")
)

// ─────────────────────────────────────────────────────────────────────────────
// Structured error type
// ─────────────────────────────────────────────────────────────────────────────

// ErrorCode is a machine-readable classification of a provider error.
// It maps cleanly to HTTP status codes via HTTPStatus().
type ErrorCode string

const (
	CodeNotConnected  ErrorCode = "NOT_CONNECTED"
	CodeNotFound      ErrorCode = "NOT_FOUND"
	CodeUnsupported   ErrorCode = "UNSUPPORTED"
	CodeAuthFailed    ErrorCode = "AUTH_FAILED"
	CodeTimeout       ErrorCode = "TIMEOUT"
	CodeInvalidState  ErrorCode = "INVALID_STATE"
	CodeQuotaExceeded ErrorCode = "QUOTA_EXCEEDED"
	CodeConflict      ErrorCode = "CONFLICT"
	CodeInternal      ErrorCode = "INTERNAL"
)

// ProviderError is a structured error that carries a machine-readable code,
// the provider name, the operation that failed, and an optional cause.
//
// Use New() or Wrap() to construct; use errors.As() to inspect.
type ProviderError struct {
	Code      ErrorCode
	Provider  string
	Operation string
	Message   string
	Cause     error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("provider[%s] %s: %s: %v", e.Provider, e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("provider[%s] %s: %s", e.Provider, e.Operation, e.Message)
}

func (e *ProviderError) Unwrap() error { return e.Cause }

// HTTPStatus maps the error code to the most appropriate HTTP status code.
func (e *ProviderError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAuthFailed:
		return http.StatusUnauthorized
	case CodeUnsupported:
		return http.StatusNotImplemented
	case CodeConflict:
		return http.StatusConflict
	case CodeInvalidState:
		return http.StatusUnprocessableEntity
	case CodeQuotaExceeded:
		return http.StatusPaymentRequired // 402 — closest semantic fit
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeNotConnected:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Constructors
// ─────────────────────────────────────────────────────────────────────────────

// New creates a ProviderError without a cause.
func New(providerName, operation string, code ErrorCode, msg string) *ProviderError {
	return &ProviderError{
		Code:      code,
		Provider:  providerName,
		Operation: operation,
		Message:   msg,
	}
}

// Wrap creates a ProviderError that wraps an underlying cause.
func Wrap(providerName, operation string, code ErrorCode, msg string, cause error) *ProviderError {
	return &ProviderError{
		Code:      code,
		Provider:  providerName,
		Operation: operation,
		Message:   msg,
		Cause:     cause,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers — map sentinel errors to ProviderError
// ─────────────────────────────────────────────────────────────────────────────

// IsNotFound reports whether err is (or wraps) ErrNotFound.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsUnsupported reports whether err is (or wraps) ErrUnsupported.
func IsUnsupported(err error) bool { return errors.Is(err, ErrUnsupported) }

// IsAuthFailed reports whether err is (or wraps) ErrAuthFailed.
func IsAuthFailed(err error) bool { return errors.Is(err, ErrAuthFailed) }

// IsTimeout reports whether err is (or wraps) ErrTimeout.
func IsTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// IsInvalidState reports whether err is (or wraps) ErrInvalidState.
func IsInvalidState(err error) bool { return errors.Is(err, ErrInvalidState) }

// HTTPStatusFor returns the HTTP status code for any error, falling back to
// 500 for unknown errors. It understands both ProviderError and sentinels.
func HTTPStatusFor(err error) int {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.HTTPStatus()
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAuthFailed):
		return http.StatusUnauthorized
	case errors.Is(err, ErrUnsupported):
		return http.StatusNotImplemented
	case errors.Is(err, ErrTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, ErrNotConnected):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrInvalidState):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
