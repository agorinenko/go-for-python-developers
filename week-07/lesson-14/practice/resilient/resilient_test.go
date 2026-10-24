package resilient_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"practice/resilient"
)

func fastPolicy() resilient.RetryPolicy {
	return resilient.RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
}

func TestRetrySuccessOnThirdAttempt(t *testing.T) {
	calls := 0
	fn := func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return resilient.MarkRetryable(errors.New("temporary"))
		}
		return nil
	}

	if err := resilient.Retry(context.Background(), fn, fastPolicy()); err != nil {
		t.Fatalf("Retry: unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("fn called %d times, want 3", calls)
	}
}

func TestRetryNonRetryableReturnsImmediately(t *testing.T) {
	calls := 0
	sentinel := errors.New("validation failed")
	fn := func(ctx context.Context) error {
		calls++
		return sentinel
	}

	err := resilient.Retry(context.Background(), fn, fastPolicy())
	if !errors.Is(err, sentinel) {
		t.Fatalf("Retry error = %v, want %v", err, sentinel)
	}
	if calls != 1 {
		t.Fatalf("fn called %d times, want 1 (не-retryable ошибку не повторяем)", calls)
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	calls := 0
	lastErr := errors.New("db still down")
	fn := func(ctx context.Context) error {
		calls++
		return resilient.MarkRetryable(fmt.Errorf("attempt %d: %w", calls, lastErr))
	}

	err := resilient.Retry(context.Background(), fn, resilient.RetryPolicy{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond,
	})
	if !errors.Is(err, lastErr) {
		t.Fatalf("Retry error = %v, want last attempt's error", err)
	}
	if calls != 3 {
		t.Fatalf("fn called %d times, want 3", calls)
	}
}

func TestRetryContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(ctx context.Context) error {
		return resilient.MarkRetryable(errors.New("slow dependency"))
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := resilient.Retry(ctx, fn, resilient.RetryPolicy{
		MaxAttempts: 100, BaseDelay: 50 * time.Millisecond, MaxDelay: time.Second,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Retry error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Retry вернулся через %v — отмена должна прерывать ожидание", elapsed)
	}
}

func tripBreaker(t *testing.T, cb *resilient.CircuitBreaker, threshold int) {
	t.Helper()
	for i := 0; i < threshold; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return errors.New("db down")
		})
		if errors.Is(err, resilient.ErrCircuitOpen) {
			t.Fatalf("Call %d: breaker разомкнулся раньше порога", i+1)
		}
	}
}

func TestBreakerOpensAfterThreshold(t *testing.T) {
	cb := resilient.NewCircuitBreaker(3, time.Minute)
	tripBreaker(t, cb, 3)

	if cb.State() != resilient.StateOpen {
		t.Fatalf("State = %v, want open", cb.State())
	}

	calls := 0
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, resilient.ErrCircuitOpen) {
		t.Fatalf("Call in open = %v, want ErrCircuitOpen", err)
	}
	if calls != 0 {
		t.Fatalf("fn called %d times in open state, want 0", calls)
	}
}

func TestBreakerHalfOpenSuccessCloses(t *testing.T) {
	cb := resilient.NewCircuitBreaker(1, 20*time.Millisecond)
	tripBreaker(t, cb, 1)

	// Щедрый допуск: ждём заметно дольше openTimeout.
	time.Sleep(100 * time.Millisecond)

	if cb.State() != resilient.StateHalfOpen {
		t.Fatalf("State = %v, want half-open (openTimeout истёк)", cb.State())
	}

	calls := 0
	if err := cb.Call(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("пробный Call: unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("пробный вызов должен выполниться ровно один раз, было %d", calls)
	}
	if cb.State() != resilient.StateClosed {
		t.Fatalf("State = %v, want closed после успешного пробного вызова", cb.State())
	}
}

func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := resilient.NewCircuitBreaker(1, 20*time.Millisecond)
	tripBreaker(t, cb, 1)

	time.Sleep(100 * time.Millisecond)

	if cb.State() != resilient.StateHalfOpen {
		t.Fatalf("State = %v, want half-open", cb.State())
	}

	calls := 0
	_ = cb.Call(context.Background(), func(ctx context.Context) error {
		calls++
		return errors.New("still down")
	})
	if calls != 1 {
		t.Fatalf("пробный вызов должен выполниться, было %d", calls)
	}
	if cb.State() != resilient.StateOpen {
		t.Fatalf("State = %v, want open после неудачного пробного вызова", cb.State())
	}

	// И снова вызовы не выполняются.
	after := 0
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		after++
		return nil
	})
	if !errors.Is(err, resilient.ErrCircuitOpen) {
		t.Fatalf("Call = %v, want ErrCircuitOpen", err)
	}
	if after != 0 {
		t.Fatalf("fn called %d times, want 0", after)
	}
}

func TestBreakerSuccessResetsCounter(t *testing.T) {
	cb := resilient.NewCircuitBreaker(2, time.Minute)

	fail := func(ctx context.Context) error { return errors.New("boom") }
	ok := func(ctx context.Context) error { return nil }

	_ = cb.Call(context.Background(), fail)
	if err := cb.Call(context.Background(), ok); err != nil {
		t.Fatalf("Call(ok): %v", err)
	}
	_ = cb.Call(context.Background(), fail) // счётчик сброшен успехом — не порог

	if cb.State() != resilient.StateClosed {
		t.Fatalf("State = %v, want closed (успех сбрасывает счётчик)", cb.State())
	}
}

func TestBreakerIgnoresCancellation(t *testing.T) {
	cb := resilient.NewCircuitBreaker(1, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // клиент отменил сам — это не отказ зависимости

	err := cb.Call(ctx, func(ctx context.Context) error {
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Call = %v, want context.Canceled", err)
	}
	if cb.State() != resilient.StateClosed {
		t.Fatalf("State = %v, want closed (отмена не размыкает цепь)", cb.State())
	}
}
