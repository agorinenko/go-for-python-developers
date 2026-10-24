# Практика. Урок 12: миграции и интеграционные тесты

Интеграционные тесты против **реальной PostgreSQL** в Docker
(testcontainers). Пакет `repository` — готовая реализация из практики
урока 11 (здесь она инструмент, а не задание); TODO ровно одна —
миграция `0002_create_outbox.sql`.

## Задание

1. **`migrations/0002_create_outbox.sql`** — напишите DDL таблицы
   `outbox` по контракту [CAPSTONE.md](../../../CAPSTONE.md) (раздел
   «Неделя 6»):
   - `id bigserial PRIMARY KEY`;
   - `event_id uuid NOT NULL`;
   - `event_type text NOT NULL`;
   - `payload jsonb NOT NULL`;
   - `created_at timestamptz NOT NULL`;
   - `published_at timestamptz` — **без NOT NULL**: NULL здесь признак
     «событие ещё не опубликовано» (transactional outbox, неделя 8);
   - секция `-- +goose Down` — `DROP TABLE outbox;`.
2. Запустите `go test ./...` — тесты поднимут PostgreSQL 16,
   применят миграции через goose (встраиваются через `embed.FS` в
   `migrations/migrations.go`) и проверят repository против живой БД.
3. Если Docker недоступен — тесты сами пропустятся с пояснением
   (`t.Skip`). В CI это недопустимо: интеграционные тесты должны
   гоняться всегда, поэтому Docker обязателен (см. README курса).

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное (при работающем Docker — без skip).

## Перенос в капстоун

1. Скопируйте `migrations/` (включая `0002` с вашим DDL и
   `migrations.go` с embed) в `orders/internal/migrations/` капстоуна —
   это контракт «Неделя 6» (плюс outbox, который задействует неделя 8).
2. Скопируйте `repository/repository.go` в
   `orders/internal/storage/postgres/`, заменив `practice/domain` →
   `orders/internal/domain`, `domain.ListFilter` → `service.ListFilter`,
   и переведите на `pgxpool` (лекция урока 11): поле `pool *pgxpool.Pool`,
   вызовы без `Context`-суффиксов. Юнит-тесты урока 11 перенесите туда же
   (они на sqlmock, отдельно от интеграционных).
3. Перенесите `integration_test.go` в пакет `postgres` капстоуна
   (в `orders/internal/storage/postgres/`), заменив импорты; в
   `Makefile` добавьте цель `test-integration: go test ./internal/...`.
   Итог: `make test` в капстоуне гоняет юнит- и интеграционные тесты
   против контейнера.
