# Практика. Урок 11: SQL в Go — репозиторий заказов на database/sql

Пакет `repository` — реализация `domain.OrderRepository` капстоуна
(контракт «Неделя 6»: `internal/storage/postgres`) поверх `*sql.DB`.
Тесты — на `go-sqlmock` (эмуляция драйвера БД в памяти, без реального
PostgreSQL; против реальной БД будем тестировать в уроке 12).

## Схема (зафиксирована, менять нельзя)

Две таблицы — заказ и позиции отдельно (а не JSON-поле в orders):
отдельная таблица честнее с точки зрения SQL (индексы по позициям,
референциальная целостность) и совпадает с подходом капстоуна.

```sql
CREATE TABLE orders (
    id          text PRIMARY KEY,
    customer_id text NOT NULL,
    status      text NOT NULL,
    total       bigint NOT NULL,
    created_at  timestamptz NOT NULL
);

CREATE TABLE order_items (
    order_id   text NOT NULL REFERENCES orders(id),
    product_id text NOT NULL,
    name       text NOT NULL,
    quantity   integer NOT NULL,
    price      bigint NOT NULL
);
```

`domain` — готовая учебная копия (недели 1–2). `domain.ListFilter` здесь
в `domain`; в капстоуне он объявлен в `internal/service` — при переносе
замените тип (поля те же).

## Задание

Реализуйте `TODO` в `repository/repository.go` (всё — в лекции):

1. **`Save`** — вставка заказа и позиций **в одной транзакции**
   (`BeginTx` + `defer tx.Rollback()` + `Commit`).
2. **`GetByID`** — заказ + позиции; `sql.ErrNoRows` → `domain.ErrOrderNotFound`.
3. **`List`** — динамический WHERE по контракту в комментарии функции:
   порядок условий фиксирован (customer_id, status), всегда
   `ORDER BY created_at DESC LIMIT $N`, `Limit <= 0` → 50, пустой
   результат — пустой не-nil слайс.
4. **`UpdateStatus`** — `RowsAffected() == 0` → `ErrOrderNotFound`.

Везде: ошибки оборачивать через `fmt.Errorf("...: %w", err)`, контекст
передавать в `*Context`-варианты методов. Именованные типы
(`domain.OrderStatus`) конвертируйте в `string` при передаче аргументов —
драйвер принимает только базовые типы.

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в капстоун

1. Скопируйте `repository/repository.go` в `orders/internal/storage/postgres/`
   капстоуна, заменив импорт `practice/domain` → `orders/internal/domain`
   и `domain.ListFilter` → `service.ListFilter` в сигнатуре `List`.
2. В капстоуне замените `*sql.DB` на `*pgxpool.Pool` (лекция, раздел
   «pgx/pgxpool»): сигнатуры методов не меняются, меняется тип поля и
   вызовы — `pgxpool` сам предоставляет `Exec/Query/QueryRow` с
   контекстом и плейсхолдерами `$1` (как `database/sql`, но без
   `QueryRowContext`-суффиксов). Миграции таблиц — в уроке 12.
3. Подключите репозиторий в `cmd/orders`: пул → `NewPostgresOrderRepository`
   → сервисный слой; добейтесь `go test ./... && go vet ./...`.
