# Практика. Урок 16: transactional outbox и saga

Пакеты практики:

- `outbox` — реле outbox: `Message`, интерфейсы `Store`/`Transport`, `Relay`
  с методами `Run` (фоновый цикл) и `ProcessOnce` (один цикл fetch → send →
  mark);
- `dedupe` — потокобезопасная дедупликация `Seen` для идемпотентного
  консьюмера саги payments.

Контракт таблицы outbox заморожен в [CAPSTONE.md](../../../CAPSTONE.md),
неделя 6: `outbox` (`id bigserial PK`, `event_id uuid`, `event_type text`,
`payload jsonb`, `published_at timestamptz NULL` = ещё не опубликовано).
Публикация в итоговом проекте — только через outbox (неделя 8).

## Задание

Реализуйте `TODO`:

1. `outbox.Relay.ProcessOnce` (`outbox/outbox.go`) — один цикл реле:
   - `FetchPending` → по одному `transport.Send`;
   - **частичный прогресс**: при сбое отправки пометьте уже успешно
     отправленные id и верните `(число успешных, обёрнутая ошибка)` —
     неотправленные остаются pending и уйдут в следующем цикле;
   - если всё отправлено — `MarkPublished` всех id; ошибка mark не отменяет
     того, что события доставлены;
   - ошибки оборачивайте через `fmt.Errorf("...: %w", err)` — тесты
     проверяют их через `errors.Is`.
2. `outbox.Relay.Run` — бесконечный цикл: `time.NewTicker(interval)`,
   выход по `ctx.Done()`, ошибки `ProcessOnce` не останавливают реле.
3. `dedupe.Seen.Once` — первый вызов ключа возвращает `true`, повторные —
   `false`; потокобезопасно (мьютекс + map).

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное. Тесты не требуют Kafka и БД — хранилище и транспорт фейковые.

## Перенос в итоговый проект

1. Скопируйте `outbox/outbox.go` в `orders/internal/events/outbox.go`
   (пакет `events`): тип `Message` дополните полями из полной схемы
   (`EventID`, `CreatedAt`, `PublishedAt`), `Store` реализуйте на
   `pgx` поверх таблицы `outbox` из миграции `0002_create_outbox.sql`:
   - `FetchPending`: `SELECT ... WHERE published_at IS NULL ORDER BY id
     LIMIT $1`;
   - `MarkPublished`: `UPDATE outbox SET published_at = now()
     WHERE id = ANY($1)`.
2. В `CreateOrder` итогового проекта замените прямую публикацию (урок 15) на запись
   в outbox **в той же транзакции**, что и заказ.
3. `dedupe/dedupe.go` перенесите в сагу `payments` (сервис-консьюмер из
   CAPSTONE.md): перед обработкой события проверяйте
   `seen.Once(orderID)` — повторное событие (at-least-once!) пропускается.
4. `Transport` из урока 15 (`internal/events`) переиспользуйте как транспорт
   реле — сигнатуры совместимы.
