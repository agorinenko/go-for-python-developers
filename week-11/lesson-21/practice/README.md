# Практика. Урок 21: rate limiting и backpressure

Пакет `ratelimit` — будущий `orders/internal/ratelimit` капстоуна. Контракт
(интерфейс `Limiter`, сигнатуры `NewTokenBucket`, `NewMiddleware`, `RunWithLimit`)
зафиксирован: менять его нельзя.

## Задание

Реализуйте три `TODO` в `ratelimit/limiter.go`:

1. **`(*TokenBucket).Allow`** — делегируйте вызов внутреннему `*rate.Limiter`
   (лекция, раздел «Token bucket»). Без блокировок и ожиданий.
2. **`NewMiddleware`** — если `limiter.Allow()` вернул `true` — вызвать `next`;
   иначе выставить `Retry-After: 1` и ответить `http.StatusTooManyRequests`,
   **не вызывая** `next` (лекция, раздел «Rate limiter как middleware»).
3. **`RunWithLimit`** — запустить все `jobs`, но не более `limit` одновременно,
   через `errgroup.WithContext` + `SetLimit` (лекция, раздел «Ограничение
   внутренней конкурентности»). Требования:
   - первая ошибка возвращается из `g.Wait()`;
   - после отмены `ctx` новые jobs не стартуют;
   - уже запущенные jobs, которые проверяют `ctx`, завершаются (общий
     контекст отменяется при первой ошибке автоматически).

Зависимости уже подключены: `golang.org/x/time`, `golang.org/x/sync`
(запустите `go mod tidy` после правок).

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное. В тестах нет тайминговых допущений: `TokenBucket` проверяется
burst-поведением (rate 1000/с, burst 2 — третий вызов отклонён), лимит
одновременности — атомарным счётчиком, а не таймерами.

## Перенос в капстоун

1. Создайте `orders/internal/ratelimit/` и скопируйте туда `ratelimit/limiter.go`
   (пакет остаётся `ratelimit`, импорты не меняются).
2. Добавьте зависимости `golang.org/x/time` и `golang.org/x/sync` в
   `orders/go.mod` (`go mod tidy` внутри капстоуна).
3. В `orders/internal/httpapi` вставьте middleware в цепочку **после recovery,
   до маршрутизатора**; ключ лимитера — IP клиента (`r.RemoteAddr`), бюджет —
   из конфига (`ratePerSec`, `burst`), в тестах httpapi используйте фейковый
   `Limiter` (`allowAll`/`denyAll` из практики).
4. Перенесите `RunWithLimit` в сервисный слой: ограничьте конкурентную
   валидацию позиций (неделя 3) значением из конфига.
5. Проверьте в капстоуне: `go test ./...` зелёный, 429 отдаётся с
   `Retry-After` и единой схемой ошибки `{"error": {...}}` (неделя 5).
