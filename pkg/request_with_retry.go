package pkg

import (
	"context"
	"errors"
	"time"
)

var (
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
)

const (
	// attemptCount defines how many attempts will be made.
	attemptCount = 5
	// retryStartTimeout is the initial timeout for a single request attempt.
	retryStartTimeout = time.Second
	// retryBackoffFactor is the multiplier applied to the timeout after each failure.
	retryBackoffFactor = 1.4
)

type (
	// requestFn represents a single request attempt.
	// The request must respect the provided context and return promptly
	// when the context is canceled or its deadline is exceeded.
	requestFn[ResponseT any] = func(tCtx context.Context) (ResponseT, error)
	// retryPostActionFn is an optional hook that is called after each failed attempt.
	// It can be used for logging, metrics, or backoff-related side effects.
	retryPostActionFn = func(retryNumber int)
)

// RequestWithRetry executes a request with retries.
// The request is retried sequentially with a per-attempt timeout.
// If the parent context is canceled, execution stops immediately.
// The request function must respect the provided context.
func RequestWithRetry[ResponseT any](
	ctx context.Context,
	request requestFn[ResponseT],
	retryPostAction retryPostActionFn,
) (ResponseT, error) {
	var zero ResponseT

	timeout := retryStartTimeout

	for i := range attemptCount {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		if i > 0 {
			retryPostAction(i)
		}

		tCtx, cancel := context.WithTimeout(ctx, timeout)

		resp, err := request(tCtx)
		cancel()
		if err == nil {
			return resp, nil
		}

		timeout = time.Duration(float64(timeout) * retryBackoffFactor)
	}

	return zero, ErrMaxRetriesExceeded
}
