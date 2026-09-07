package ports

import (
	"context"
	"errors"
	"net"

	"gust/pkg/api"
)

// ErrTransient marks a runner error as a retryable infrastructure blip
// (network, provider hiccup) rather than an agent execution failure.
var ErrTransient = errors.New("transient runner error")

// IsRetryable reports whether err should be retried under the given policy.
// context.Canceled is never retried. DeadlineExceeded is retried only for RetryOnAll.
func IsRetryable(err error, on api.RetryOn) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	switch on {
	case api.RetryOnNone:
		return false
	case api.RetryOnAll:
		return true
	default:
		if errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		if errors.Is(err, ErrTransient) {
			return true
		}
		var ne net.Error
		if errors.As(err, &ne) {
			return true
		}
		return false
	}
}
