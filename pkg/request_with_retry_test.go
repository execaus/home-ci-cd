package pkg

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRequestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	request := func(tCtx context.Context) (string, error) {
		attempts++
		return "success", nil
	}

	var calledRetries []int
	retryPostAction := func(retryNumber int) {
		calledRetries = append(calledRetries, retryNumber)
	}

	resp, err := RequestWithRetry(ctx, request, retryPostAction)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "success" {
		t.Fatalf("expected success response, got %v", resp)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if len(calledRetries) != 0 {
		t.Fatalf("expected no retry post actions called, got %d", len(calledRetries))
	}
}

func TestRequestWithRetry_EventualSuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	request := func(tCtx context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("fail")
		}
		return "success", nil
	}

	var calledRetries []int
	retryPostAction := func(retryNumber int) {
		calledRetries = append(calledRetries, retryNumber)
	}

	resp, err := RequestWithRetry(ctx, request, retryPostAction)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "success" {
		t.Fatalf("expected success response, got %v", resp)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(calledRetries) != 2 {
		t.Fatalf("expected 2 retry post actions called, got %d", len(calledRetries))
	}
}

func TestRequestWithRetry_ExceedsMaxRetries(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	request := func(tCtx context.Context) (string, error) {
		attempts++
		return "", errors.New("fail")
	}

	var calledRetries []int
	retryPostAction := func(retryNumber int) {
		calledRetries = append(calledRetries, retryNumber)
	}

	_, err := RequestWithRetry(ctx, request, retryPostAction)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("expected ErrMaxRetriesExceeded, got %v", err)
	}
	if attempts != attemptCount {
		t.Fatalf("expected %d attempts, got %d", attemptCount, attempts)
	}
	if len(calledRetries) != attemptCount-1 {
		t.Fatalf("expected %d retry post actions called, got %d", attemptCount-1, len(calledRetries))
	}
}

func TestRequestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	request := func(tCtx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return "", errors.New("fail")
	}

	var calledRetries []int
	retryPostAction := func(retryNumber int) {
		calledRetries = append(calledRetries, retryNumber)
	}

	_, err := RequestWithRetry(ctx, request, retryPostAction)
	if err == nil {
		t.Fatalf("expected an error due to context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if len(calledRetries) != 0 {
		t.Fatalf("expected no retry post actions called, got %d", len(calledRetries))
	}
}

func TestRequestWithRetry_RequestHonorsContextTimeout(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	request := func(tCtx context.Context) (string, error) {
		attempts++
		if attempts < attemptCount {
			select {
			case <-time.After(2 * time.Second):
				return "success", nil
			case <-tCtx.Done():
				return "", tCtx.Err()
			}
		}
		return "success", nil
	}

	var calledRetries []int
	retryPostAction := func(retryNumber int) {
		calledRetries = append(calledRetries, retryNumber)
	}

	resp, err := RequestWithRetry(ctx, request, retryPostAction)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "success" {
		t.Fatalf("expected success response, got %v", resp)
	}
	if attempts != attemptCount {
		t.Fatalf("expected %d attempts, got %d", attemptCount, attempts)
	}
	if len(calledRetries) != attemptCount-1 {
		t.Fatalf("expected %d retry post actions called, got %d", attemptCount-1, len(calledRetries))
	}
}
