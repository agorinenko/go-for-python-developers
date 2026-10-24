// Package ratelimit — ограничение входящего потока (token bucket) и
// внутренней конкурентности (errgroup.SetLimit).
// Контракт зафиксирован: сигнатуры и имена экспортируемых элементов менять нельзя.
package ratelimit

import (
	"context"
	"net/http"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Limiter — минимальный контракт ограничителя входящего потока.
// Реализации: TokenBucket (ниже), а в тестах и капстоуне — фейки и
// централизованные лимитеры поверх Redis.
type Limiter interface {
	// Allow сообщает, можно ли пропустить запрос прямо сейчас.
	Allow() bool
}

// TokenBucket — token bucket поверх golang.org/x/time/rate:
// ведро ёмкостью burst пополняется со скоростью ratePerSec токенов в секунду.
type TokenBucket struct {
	limiter *rate.Limiter
}

// NewTokenBucket создаёт TokenBucket с частотой ratePerSec токенов/сек
// и пиковой ёмкостью burst («кредит» на всплески).
func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket {
	return &TokenBucket{limiter: rate.NewLimiter(rate.Limit(ratePerSec), burst)}
}

// Allow сообщает, есть ли токен на запрос прямо сейчас. Не блокируется:
// нет токена — запрос отклоняется немедленно (middleware отдаст 429).
//
// TODO: делегируй вызов внутреннему *rate.Limiter.
func (tb *TokenBucket) Allow() bool {
	// TODO: верни tb.limiter.Allow().
	return false
}

// retryAfterSeconds — что писать в заголовок Retry-After при отказе.
// Для простого лимитера интерфейс не раскрывает расписание пополнения,
// поэтому подсказка клиенту фиксированная.
const retryAfterSeconds = "1"

// NewMiddleware возвращает middleware, который пропускает запрос дальше,
// если limiter.Allow() вернул true, и отвечает 429 + Retry-After иначе.
//
// TODO: реализуй:
//  1. limiter.Allow() == true → next.ServeHTTP(w, r);
//  2. иначе w.Header().Set("Retry-After", retryAfterSeconds) и
//     w.WriteHeader(http.StatusTooManyRequests) — next вызывать НЕЛЬЗЯ.
func NewMiddleware(limiter Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: реализуй по описанию выше.
		// Заглушка: пропускаем всё — тест на 429 сейчас падает.
		next.ServeHTTP(w, r)
	})
}

// RunWithLimit запускает все jobs, но не более limit одновременно.
// Возвращает первую ошибку; после отмены ctx новые jobs не стартуют,
// а уже запущенные, проверяющие ctx, завершаются.
//
// TODO: реализуй через golang.org/x/sync/errgroup:
//  1. g, ctx := errgroup.WithContext(ctx) — общий контекст отменяется при первой ошибке;
//  2. g.SetLimit(limit) — встроенный семафор: g.Go блокируется, пока все слоты заняты;
//  3. в цикле по jobs вызывай g.Go(func() error { ... }); внутри горутины первым делом
//     проверь ctx.Err() и верни его, если ctx уже отменён — так воркер не делает работу
//     после отмены (сам вызов g.Go заблокирован на семафоре и ctx не проверяет);
//  4. верни g.Wait() — первую ошибку (или nil).
func RunWithLimit(ctx context.Context, limit int, jobs []func() error) error {
	// TODO: реализуй по описанию выше.
	// Заглушка: jobs выполняются без ограничения одновременности, ошибки
	// игнорируются, ctx не используется — тесты на лимит и отмену сейчас падают.
	var g errgroup.Group
	for _, job := range jobs {
		g.Go(func() error {
			job()
			return nil
		})
	}
	return g.Wait()
}
