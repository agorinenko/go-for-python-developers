# Урок 11. SQL в Go: database/sql, pgx, транзакции, sqlmock

## Цели урока

- освоить `database/sql`: пул соединений и его настройка, подготовленные
  выражения, транзакции, обработка `NULL` и `sql.ErrNoRows`;
- понять, почему для PostgreSQL выбирают pgx/pgxpool вместо
  стандартного драйвера;
- строить запросы с плейсхолдерами `$1` и понимать, чем это отличается
  от конкатенации (SQL-инъекции);
- тестировать SQL-слой через sqlmock.

## Задача из капстоуна

Контракт «Неделя 6» (CAPSTONE.md): `internal/storage/postgres` —
реализация `domain.OrderRepository` на PostgreSQL. Сегодня пишем
репозиторий поверх `*sql.DB` и покрываем его sqlmock-тестами; в уроке 12
перенесём его на реальную БД в testcontainers с миграциями.

## database/sql: пул соединений

`database/sql` — это **не драйвер**, а общий слой над драйверами
(`lib/pq`, `pgx/stdlib`, `go-sqlite3`). Он даёт: пул соединений,
управление транзакциями, подготовленные выражения, единый API для всех
СУБД.

Пул настраивается на `*sql.DB` (который и есть пул, а не соединение):

```go
db.SetMaxOpenConns(25)           // максимум открытых соединений
db.SetMaxIdleConns(25)           // сколько держать про запас
db.SetConnMaxLifetime(5 * time.Minute)  // возраст соединения
db.SetConnMaxIdleTime(5 * time.Minute)  // сколько idle до закрытия
```

Почему это критично: дефолт `MaxOpenConns` — **неограниченно**. Под
нагрузкой сервис откроет столько соединений, сколько запросов, и положит
базу (у PostgreSQL лимит — сотни). Разумная формула: `MaxOpenConns` ≈
`core count * 2–4`; `MaxIdleConns = MaxOpenConns` (соединения — дорогие,
пусть живут); `ConnMaxLifetime` — меньше, чем таймауты сетевого
оборудования/балансировщика перед БД, иначе получите `unexpected EOF`
от прокси, убившего протухшее соединение.

`db.PingContext` при старте — обязательный health-check: ошибка
конфигурации DSN должна всплыть на деплое, а не на первом запросе.

## Запросы, контексты, плейсхолдеры

```go
var total int64
err := db.QueryRowContext(ctx,
    "SELECT total FROM orders WHERE id = $1", id).Scan(&total)
```

Правила, которые отличают Go-код от привычного:

1. **Всегда `*Context`-варианты** (`QueryContext`, `ExecContext`).
   Методы без суффикса не принимают контекст — вызов уйдёт в БД без
   таймаута и отмены. Контекст из HTTP-запроса (урок 9) пробрасывается
   до самого драйвера: клиент отвалился — запрос в базе отменён.
2. **Плейсхолдеры, не интерполяция.** PostgreSQL — `$1, $2, ...`; никогда
   не собирайте SQL строковой конкатенацией значений — это и есть
   инъекция (`"WHERE id = '" + id + "'"`). Драйвер отправляет параметры
   отдельно от текста запроса (extended protocol), и данные физически не
   могут стать кодом.
3. **`Scan` копирует значения в переменные** по типам колонок. Типы
   `time.Time`, `int64`, `string` — прямые; составные (jsonb, массивы) —
   через драйвер или `[]byte` + ручной разбор.

`sql.ErrNoRows` — сигнал «пусто», не ошибка-исключение: его мапят в
доменный sentinel:

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, domain.ErrOrderNotFound
}
```

## NULL-обработка

Go не умеет хранить NULL в `string`/`int64` — отсюда обёртки:
`sql.NullString`, `sql.NullInt64`, `sql.NullTime` (структуры с полями
`Valid bool` и значением). Два подхода:

- `sql.NullString` — scan в обёртку, потом `if s.Valid {...}`;
- `NULLIF`/`COALESCE` в SQL — вытащить NULL в дефолт (`COALESCE(published_at, 'epoch')`), когда «не было» и «пусто» семантически одно.

Для нашего outbox-поля `published_at` (урок 12) NULL — **смысл**: «ещё не
опубликовано». Тогда `sql.NullTime` честнее, чем магический `0001-01-01`.

## Транзакции

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return fmt.Errorf("begin tx: %w", err) }
defer func() { _ = tx.Rollback() }() // после Commit — ErrTxDone, игнорируем

if _, err := tx.ExecContext(ctx, insertOrderSQL, ...); err != nil {
    return fmt.Errorf("insert order: %w", err)
}
for _, it := range o.Items {
    if _, err := tx.ExecContext(ctx, insertItemSQL, ...); err != nil {
        return fmt.Errorf("insert item %s: %w", it.ProductID, err)
    }
}
return tx.Commit()
```

Паттерн **`defer Rollback` + явный Commit** — стандарт: любой ранний
return откатывает транзакцию, успешный путь коммитит. Игнорировать ошибку
`Rollback` в defer допустимо только потому, что путь успеха уже
закоммичен (там Rollback вернёт `ErrTxDone`); на пути ошибки откат —
best effort, настоящую ошибку мы уже вернули.

Изоляция: `BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})`.
Дефолт PostgreSQL — read committed, и для нашего кейса (заказ + позиции в
одной транзакции) его достаточно. Изоляцию поднимайте осознанно: выше
изоляция — больше блокировок и retries на `serialization_failure`.

## Динамические запросы (фильтры)

`List` с опциональными фильтрами — запрос собирается кодом, значения —
отдельно:

```go
query := baseListSQL
args := make([]any, 0, 3)
if f.CustomerID != "" {
    query += " WHERE customer_id = $" + strconv.Itoa(len(args)+1)
    args = append(args, f.CustomerID)
}
query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1)
args = append(args, limit)
```

Три правила: условия добавляются в **фиксированном порядке** (тесты
проверяют аргументы), номер плейсхолдера = `len(args)+1`, значения —
только через args. Интерполяция значений в текст запроса здесь так же
запрещена, как и в статических запросах.

## pgx и pgxpool: почему не стандартный драйвер

`github.com/jackc/pgx/v5` — нативный драйвер PostgreSQL (не через
`database/sql`):

- **pgxpool** — свой пул с health-check соединений (битые соединения
  пересоздаются, а не отдаются в работу);
- `database/sql` + `lib/pq` — каждый вызов `QueryContext` возвращает
  соединение в пул по закрытию `rows`; забыли закрыть — утечка соединений;
  pgxpool на это честнее ругается;
- **типы**: pgx понимает PostgreSQL-типы напрямую (jsonb → структуры,
  массивы, uuid), без `sql.Null*` прослоек — NULL сканируется в
  `*string`/`sql.NullTime` аккуратнее;
- **batch-запросы** (`pgx.Batch`) и copy-протокол — для массовой вставки;
- плейсхолдеры те же `$1`, сигнатуры почти те же, миграция с
  `database/sql` на pgx — механическая.

Компромисс: pgx поддерживает и режим `database/sql` (пакет
`pgx/v5/stdlib`) — именно его берём в интеграционных тестах урока 12,
чтобы не зависеть от двух API одновременно.

**sqlc** — генератор типизированного Go-кода из SQL-файлов: пишете
`-- name: GetOrder :one` над запросом, получаете функцию с готовой
сигнатурой и типами. Это инверсия ORM: источник истины — SQL, а не
модель. Для нашего сервиса запросов мало — пишем руками; при десятке
таблиц sqlc экономит и время, и баги опечаток в Scan.

## sqlmock: тестирование SQL-слоя

`github.com/DATA-DOG/go-sqlmock` подменяет драйвер `database/sql`
реализацией в памяти: тест задаёт **ожидания** (запрос, аргументы,
результат) и проверяет, что репозиторий выполнил ровно это.

```go
db, mock, _ := sqlmock.New()
mock.ExpectBegin()
mock.ExpectExec(regexp.QuoteMeta(repository.InsertOrderSQL)).
    WithArgs("o-1", "c-1", "new", int64(2250), createdAt).
    WillReturnResult(sqlmock.NewResult(0, 1))
mock.ExpectCommit()

err := repo.Save(context.Background(), o)   // выполняется против mock
mock.ExpectationsWereMet()                  // все ожидания совпали
```

Что sqlmock проверяет: порядок вызовов, текст SQL (регуляркой — поэтому
`regexp.QuoteMeta`), значения аргументов, ошибки (`WillReturnError`) и
строки (`NewRows(...).AddRow(...)`). Ошибка в середине транзакции +
`ExpectRollback` проверяет ровно то, ради чего писался `defer Rollback`.

Чего sqlmock **не** проверяет: синтаксис SQL, реальное поведение СУБД,
типы колонок. Опечатку в имени таблицы sqlmock пропустит, а PostgreSQL —
нет. Поэтому sqlmock — юнит-слой; реальную БД добавит урок 12
(testcontainers). Дополняют друг друга, не заменяют.

## Go vs Python: SQLAlchemy и «тонкий слой против ORM»

```python
# SQLAlchemy ORM: модель = таблица
class Order(Base):
    id: Mapped[str] = mapped_column(primary_key=True)
    items: Mapped[list["OrderItem"]] = relationship()

session.add(order)          # unit of work: сам решит, что INSERTить
await session.flush()
```

SQLAlchemy — мощный ORM: связи, unit of work, lazy loading, async-сессия.
Плата: «магия» — когда и какой SQL улетит в базу, решает ORM; N+1-запросы
вылезают под нагрузкой; отладка требует чтения сгенерированного SQL.

Go-сообщество в основном выбирает **тонкий слой над SQL**: вы пишете
запросы текстом, репозиторий — это функции + транзакции. Причины:

1. SQL — уже декларативный язык с отличной экосистемой; ORM поверх него —
   второй язык, который надо учить и отлаживать.
2. В микросервисе на сервис — 2–5 таблиц; автогенерация схемы и
   миграции связей не окупаются.
3. Производительность предсказуема: что написано — то и выполнилось.
   В Go не нужно бояться блокирующих вызовов драйвера (горутины), тогда
   как в Python блокирующий SQLAlchemy в async-коде вешает event loop —
   отсюда пляски с async-сессиями и пулами.

**C#**: EF Core — аналог SQLAlchemy с теми же компромиссами; Dapper —
аналог тонкого слоя. Go в этой оси ближе к Dapper-краю, но обычно даже
без микро-ORM: `database/sql`/pgx уже покрывает «SQL + Scan».

## Подводные камни и анти-паттерны

1. **Конкатенация значений в SQL.** `fmt.Sprintf("... %s", id)` — инъекция.
   Только плейсхолдеры. Исключение — идентификаторы (имена таблиц),
   их нельзя параметризовать; тогда — белый список допустимых значений.
2. **Ненастроенный пул.** Без `SetMaxOpenConns` под нагрузкой сервис
   откроет сотни соединений и уронит базу.
3. **`Scan` без проверки ошибки** или несовпадение числа колонок —
   ошибка появляется в рантайме; sqlmock её не поймает, реальная БД — да.
4. **Незакрытые `rows`.** Соединение не вернётся в пул. Всегда
   `defer rows.Close()` + проверка `rows.Err()` после цикла.
5. **Именованные типы как аргументы.** `domain.OrderStatus` драйвер не
   примет — конвертируйте `string(status)`. Классический сюрприз первого
   коммита.
6. **`tx.Commit()` и забытый откат.** Без `defer Rollback` ошибка в
   середине оставит открытую транзакцию, которая додержит блокировки.
7. **Запросы без контекста.** `Query` вместо `QueryContext` — таймаут
   запроса не ограничен ничем, отмена клиента не пробрасывается.
8. **Дефолты как ложная экономия.** `LIMIT 0` в фильтре — «верни всё»,
   если код написан наивно. Ограничивайте сверху (у нас дефолт 50).
9. **sqlmock вместо реальной БД.** Юнит-тесты зелёные, а таблицы нет.
   Интеграционные тесты (урок 12) — обязательная вторая линия.
10. **`ORDER BY` без детерминированности** — при равных `created_at`
    порядок не гарантирован; для пагинации добавляйте вторичный ключ (id).

## Что делаем в капстоуне

1. `orders/internal/storage/postgres`: `PostgresOrderRepository`
   (Save/GetByID/List/UpdateStatus) — из практики, на pgxpool в капстоуне.
2. Юнит-тесты на sqlmock — из практики (в капстоуне импорт
   `orders/internal/domain`).
3. Урок 12 добавит миграции таблиц и интеграционные тесты против
   реальной PostgreSQL в testcontainers.

## Материалы для углубления

- [database/sql](https://pkg.go.dev/database/sql) — пул, транзакции, Null-типы.
- [Go database/sql tutorial](https://go.dev/doc/tutorial/database-access) — официальный туториал.
- [pgx](https://github.com/jackc/pgx) — драйвер PostgreSQL; pgxpool.
- [sqlc](https://sqlc.dev/) — генератор из SQL.
- [go-sqlmock](https://github.com/DATA-DOG/go-sqlmock) — документация.
- [SQLAlchemy](https://docs.sqlalchemy.org/) — ORM для сравнения.
