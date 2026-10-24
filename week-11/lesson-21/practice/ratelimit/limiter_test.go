package ratelimit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"practice/ratelimit"
)

// Фейковые лимитеры для тестов middleware — без таймингов и без сети.

type allowAll struct{}

func (allowAll) Allow() bool { return true }

type denyAll struct{}

func (denyAll) Allow() bool { return false }

func TestTokenBucketBurstThenReject(t *testing.T) {
	// rate = 1000 токенов/с, burst = 2. Вызовы идут мгновенно друг за другом,
	// поэтому пополнение не успевает: первые два запроса проходят «в кредит»,
	// третий — отклоняется. Детерминированно, без sleep.
	tb := ratelimit.NewTokenBucket(1000, 2)

	for i := 0; i < 2; i++ {
		if !tb.Allow() {
			t.Fatalf("Allow() = false на вызове %d: burst из 2 токенов должен пропускаться сразу", i+1)
		}
	}
	if tb.Allow() {
		t.Fatal("Allow() = true на 3-м вызове: токены исчерпаны, запрос должен быть отклонён")
	}
}

func TestMiddlewareAllowsRequest(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	ratelimit.NewMiddleware(allowAll{}, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: запрос должен пройти сквозь middleware", rec.Code)
	}
	if !nextCalled {
		t.Fatal("next handler не вызван для разрешённого запроса")
	}
}

func TestMiddlewareRejectsRequest(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	ratelimit.NewMiddleware(denyAll{}, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (429 Too Many Requests)", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("заголовок Retry-After = %q, want %q: клиент должен знать, когда повторить", got, "1")
	}
	if nextCalled {
		t.Fatal("next handler вызван для отклонённого запроса")
	}
}

func TestRunWithLimitMaxConcurrency(t *testing.T) {
	const (
		limit = 2
		jobsN = 8
	)

	var (
		active    atomic.Int32 // сколько воркеров сейчас внутри job
		maxActive atomic.Int32 // максимум за всё время
	)
	started := make(chan struct{}, jobsN) // «я дошёл до шлюза»
	release := make(chan struct{})        // шлюз: закрывается тестом
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	defer closeRelease() // не оставляем воркеров заблокированными при падении теста

	jobs := make([]func() error, jobsN)
	for i := range jobs {
		jobs[i] = func() error {
			cur := active.Add(1)
			for {
				m := maxActive.Load()
				if cur <= m || maxActive.CompareAndSwap(m, cur) {
					break
				}
			}
			started <- struct{}{}
			// Небольшая пауза гарантирует, что воркеры, запущенные без лимита,
			// «перекрываются» во времени — нарушение ловится счётчиком active.
			// Для корректной реализации с лимитом пауза ни на что не влияет.
			time.Sleep(10 * time.Millisecond)
			<-release
			active.Add(-1)
			return nil
		}
	}

	done := make(chan error, 1)
	go func() { done <- ratelimit.RunWithLimit(context.Background(), limit, jobs) }()

	// Ровно limit воркеров должны дойти до шлюза — остальные ждут свободного слота.
	for i := 0; i < limit; i++ {
		<-started
	}
	if got := active.Load(); got != limit {
		t.Fatalf("active = %d, want %d: лимит одновременности не соблюдён", got, limit)
	}

	closeRelease()
	if err := <-done; err != nil {
		t.Fatalf("RunWithLimit: неожиданная ошибка: %v", err)
	}
	if got := maxActive.Load(); got > limit {
		t.Fatalf("max active = %d, want <= %d", got, limit)
	}
}

func TestRunWithLimitPropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	jobs := []func() error{
		func() error { return nil },
		func() error { return wantErr },
		func() error { return nil },
	}

	err := ratelimit.RunWithLimit(context.Background(), 2, jobs)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunWithLimit error = %v, want errors.Is %v", err, wantErr)
	}
}

func TestRunWithLimitStopsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstStarted := make(chan struct{})
	var othersStarted atomic.Int32

	jobs := []func() error{
		func() error { // первый воркер висит до отмены
			firstStarted <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
		func() error { // остальные обязаны проверять ctx и не стартовать работу после отмены
			if err := ctx.Err(); err != nil {
				return err
			}
			othersStarted.Add(1)
			return nil
		},
		func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			othersStarted.Add(1)
			return nil
		},
	}

	done := make(chan error, 1)
	go func() { done <- ratelimit.RunWithLimit(ctx, 1, jobs) }()

	<-firstStarted
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWithLimit error = %v, want context.Canceled", err)
	}
	if got := othersStarted.Load(); got != 0 {
		t.Fatalf("после отмены стартовало %d воркеров, want 0", got)
	}
}
