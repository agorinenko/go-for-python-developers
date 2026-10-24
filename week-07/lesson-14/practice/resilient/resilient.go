// Package resilient — примитивы надёжных межсервисных вызовов:
// retry с backoff+jitter и circuit breaker.
// В капстоуне это orders/internal/client/catalog (неделя 7, урок 14):
// gRPC-клиент каталога, обёрнутый этими примитивами.
package resilient

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Func — операция, которую нужно сделать надёжной (вызов сервиса, запрос).
type Func func(ctx context.Context) error

// RetryableError — обёртка ошибки, которую Retry считает повторимой.
// Классификация «повторять или нет» — обязанность вызывающего кода:
// повторять безопасно только идемпотентные ошибки (таймауты, «соединение
// разорвано», 5xx-ответы без побочных эффектов). 4xx и ошибки валидации
// помечать нельзя.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	if e.Err == nil {
		return "retryable error"
	}
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error { return e.Err }

// MarkRetryable помечает ошибку как повторимую. Уже помеченную ошибку
// повторно не оборачивает (иначе IsRetryable работает, но цепочка растёт).
// Готовая функция.
func MarkRetryable(err error) error {
	var re *RetryableError
	if errors.As(err, &re) {
		return err
	}
	return &RetryableError{Err: err}
}

// IsRetryable проверяет метку (проходит сквозь любые обёртки %w).
// Готовая функция.
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

// RetryPolicy — политика повторов.
type RetryPolicy struct {
	MaxAttempts int           // всего попыток, включая первую (>= 1)
	BaseDelay   time.Duration // базовая задержка между попытками
	MaxDelay    time.Duration // потолок задержки: экспонента не взлетает
}

// Retry вызывает fn до MaxAttempts раз.
//
// Правила (лекция, раздел «Retry с backoff и jitter»):
//  1. fn вернула nil — успех, выходим;
//  2. ошибка без метки Retryable — НЕ повторяем, возвращаем сразу;
//  3. между попытками — exponential backoff с full jitter:
//     delay = случайное целое из [0, min(MaxDelay, BaseDelay * 2^(n-1))],
//     где n — номер прошедшей попытки (после первой попытки n=1);
//     full jitter обязателен: без него клиенты, стартующие одновременно,
//     долбят сервис синхронными волнами (лекция);
//  4. во время ожидания слушай ctx: отмена → верни ctx.Err() немедленно;
//  5. попытки исчерпаны — верни последнюю ошибку;
//  6. MaxAttempts < 1 — считай равным 1.
//
// TODO: реализуй (math/rand/v2 для случайного числа; time.NewTimer/select
// для ожидания с отменой). Сейчас заглушка вызывает fn один раз и
// возвращает результат как есть — «успех с 3-й попытки» падает.
func Retry(ctx context.Context, fn Func, policy RetryPolicy) error {
	// TODO: реализуй.
	return fn(ctx)
}

// State — состояние circuit breaker.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen — breaker разомкнут: вызов не выполнялся вовсе.
var ErrCircuitOpen = errors.New("resilient: circuit breaker is open")

// CircuitBreaker — автоматический выключатель вызовов (лекция, раздел
// «Circuit breaker»). Защищает сервис от каскадных отказов: когда
// зависимость лежит, вызовы к ней не выполняются вовсе.
type CircuitBreaker struct {
	mu          sync.Mutex
	threshold   int           // сколько подряд ошибок размыкает цепь
	openTimeout time.Duration // сколько пробыть open до пробного вызова
	state       State
	consecFails int
	openedAt    time.Time
}

// NewCircuitBreaker создаёт breaker в состоянии closed.
// Готовая функция: threshold < 1 приравнивается к 1.
func NewCircuitBreaker(threshold int, openTimeout time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 1
	}
	return &CircuitBreaker{
		threshold:   threshold,
		openTimeout: openTimeout,
		state:       StateClosed,
	}
}

// State возвращает эффективное состояние: open, у которого истёк
// openTimeout, сообщается как half-open. Готовая функция.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen && time.Since(cb.openedAt) >= cb.openTimeout {
		return StateHalfOpen
	}
	return cb.state
}

// Call выполняет fn через breaker (лекция, раздел «Circuit breaker»).
//
// Семантика состояний:
//
//	closed:    вызов выполняется; успех сбрасывает счётчик подряд идущих
//	           ошибок; threshold ошибок подряд → open (openedAt = now);
//	open:      вызов НЕ выполняется — немедленно возвращается
//	           ErrCircuitOpen, пока не истечёт openTimeout;
//	half-open: openTimeout истёк — выполняется ОДИН пробный вызов:
//	           успех → closed (счётчик сброшен),
//	           ошибка → снова open (openedAt = now).
//
// Ошибки context.Canceled/context.DeadlineExceeded — НЕ отказы:
// клиент сам отменил вызов, счётчик не трогаем и цепь не размыкаем.
//
// TODO: реализуй под cb.mu (учебный вариант — мутекс на весь вызов;
// промышленный вариант разблокирует fn и отдельно защищает пробный
// вызов в half-open). Сейчас заглушка всегда вызывает fn — тест
// «в open fn не вызывается» падает.
func (cb *CircuitBreaker) Call(ctx context.Context, fn Func) error {
	// TODO: реализуй.
	return fn(ctx)
}
