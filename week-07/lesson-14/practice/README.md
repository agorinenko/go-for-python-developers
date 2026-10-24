# Практика. Урок 14: надёжные вызовы — retry и circuit breaker

Пакет `resilient` — примитивы для межсервисных вызовов. В капстоуне это
`orders/internal/client/catalog` (контракт «Неделя 7»): gRPC-клиент
сервиса каталога, обёрнутый этими примитивами. Только стандартная
библиотека.

## Задание

Реализуйте `TODO` в `resilient/resilient.go`:

1. **`Retry`** — повтор вызовов с классификацией ошибок:
   - помеченные `MarkRetryable` — повторяются, остальные — возвращаются
     немедленно;
   - exponential backoff с **full jitter**: задержка — случайное целое
     из `[0, min(MaxDelay, BaseDelay * 2^(n-1))]`;
   - отмена контекста прерывает ожидание (вернуть `ctx.Err()`);
   - попытки исчерпаны — вернуть последнюю ошибку.
2. **`CircuitBreaker.Call`** — машина состояний closed/open/half-open:
   - `threshold` ошибок подряд (не считая отмен контекста) → open;
   - в open вызовы не выполняются: мгновенно `ErrCircuitOpen`;
   - по истечении `openTimeout` — half-open: один пробный вызов;
     успех → closed, ошибка → снова open;
   - под `cb.mu` (мутекс уже в структуре).

Всё, что не помечено TODO, — готовый код (метки ошибок, `State()`,
конструктор): менять не нужно.

## Критерий сдачи

```sh
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в капстоун

1. Скопируйте `resilient/resilient.go` (и тесты) в
   `orders/internal/client/catalog/` капстоуна — пакет
   `clientcatalog` (или оставьте пакет `resilient` рядом, как удобнее).
2. Напишите gRPC-клиент каталога из сгенерированного
   `api/catalog/v1/catalog.proto` (`GetProduct`) и оберните вызов:
   `Retry` с политикой «3 попытки, base 50ms, max 1s» для retryable
   ошибок (Unavailable, DeadlineExceeded — не 4xx-кодов) и
   `CircuitBreaker` с порогом 5 и openTimeout 30s, shared на всех
   вызовах.
3. Все вызовы — с дедлайном (`context.WithTimeout`) и пробросом
   контекста HTTP/gRPC-запроса. Добейтесь `go test ./... && go vet ./...`
   в капстоуне.
