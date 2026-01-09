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
	retryAttemptCount  = 5
	retryStartTimeout  = time.Millisecond * 300
	retryBackoffFactor = 1.5
)

func RequestWithRetry[ResponseT any](ctx context.Context, request func() (ResponseT, error)) (ResponseT, error) {
	// TODO
	return *new(ResponseT), nil
}
