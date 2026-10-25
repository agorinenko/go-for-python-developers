# Итоговый проект: сервис заказов «Orders»

Сквозной проект курса. Цель: к концу 12-й недели получить микросервисную систему,
готовую к развёртыванию в Kubernetes. Проект растёт строго по неделям — каждый урок
добавляет к нему часть, определённую этим контрактом.

## Итоговая архитектура

```
                    ┌──────────────┐   gRPC    ┌───────────┐
                    │    catalog   │◄──────────│           │
                    └──────────────┘           │  orders   │
                    ┌──────────────┐  REST/    │  (Go)     │
                    │   клиенты    │──►HTTP+gRPC│           │
                    └──────────────┘           └──┬───┬───┬─┘
                                                 │   │   │
                           ┌─────────────────────┘   │   └──────────────┐
                           ▼                         ▼                  ▼
                    ┌────────────┐          ┌──────────────┐   ┌─────────────┐
                    │ PostgreSQL │          │     Kafka    │   │    Redis    │
                    │ (orders,   │          │ orders.created│  │  (кэш заказов)│
                    │  outbox)   │          └──────┬───────┘   └─────────────┘
                    └────────────┘                 ▼
                                          ┌────────────────┐
                                          │payments (saga) │
                                          └────────────────┘
```

- **orders** — сервис заказов (наш итоговый проект): принимает заказы по HTTP и gRPC,
  хранит в PostgreSQL, публикует события в Kafka, кэширует в Redis.
- **catalog** — небольшой сервис каталога (неделя 7): заглушка-микросервис на Go,
  который orders вызывает по gRPC, чтобы проверить цены и наличие товара.
- **payments** — консьюмер событий (неделя 8): saga «оплата заказа» поверх outbox.

## Расположение и модуль

Итоговый проект создаётся на уроке 1 в каталоге `orders/` в корне репозитория курса.
Модуль называется `orders`:

```
orders/
├── go.mod                  # module orders
├── Makefile                # build / run / test / lint / fmt
├── .golangci.yml
├── cmd/orders/main.go      # точка входа
├── internal/
│   ├── domain/             # Order, OrderItem, OrderStatus, ошибки (неделя 1)
│   ├── service/            # OrderService (неделя 2)
│   ├── cache/              # OrderCache (недели 4, 11)
│   ├── httpapi/            # REST API, DTO, middleware (неделя 5)
│   ├── storage/postgres/   # repository на PostgreSQL (неделя 6)
│   ├── migrations/         # goose-миграции (неделя 6)
│   ├── grpcapi/            # gRPC-сервер (неделя 7)
│   ├── client/catalog/     # клиент сервиса каталога (неделя 7)
│   ├── events/             # продюсер событий, outbox-реле (неделя 8)
│   ├── telemetry/          # log, metrics, tracing (неделя 10)
│   └── ratelimit/          # rate limiter (неделя 11)
├── api/orders/v1/orders.proto     # контракт gRPC (неделя 7)
├── deployments/
│   ├── docker/             # Dockerfile, compose (неделя 12)
│   ├── k8s/                # манифесты (неделя 12)
│   └── ci/                 # конвейер (неделя 12)
└── README.md
```

## Замороженные контракты

Контракты ниже **замораживаются** в указанную неделю: последующие уроки опираются
на них как на данность. Отступления возможны, но тогда придётся править код
последующих недель — контракт существует, чтобы этого избежать.

### Неделя 1 — доменная модель (`internal/domain`)

```go
type OrderStatus string

const (
    StatusNew       OrderStatus = "new"
    StatusValidated OrderStatus = "validated"
    StatusPaid      OrderStatus = "paid"
    StatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
    ProductID string
    Name      string
    Quantity  int
    Price     int64 // в копейках (никогда не float!)
}

type Order struct {
    ID         string
    CustomerID string
    Items      []OrderItem
    Status     OrderStatus
    Total      int64 // в копейках
    CreatedAt  time.Time
}
```

Допустимые переходы статусов: `new → validated → paid`, `new → cancelled`.
Ошибки домена: `ErrEmptyCustomer`, `ErrNoItems`, `ErrInvalidItem`,
`ErrOrderNotFound`, `ErrInvalidTransition` — sentinel-значения, проверяются через
`errors.Is`.

### Неделя 2 — сервисный слой (`internal/service`)

```go
type ListFilter struct {
    CustomerID string
    Status     domain.OrderStatus
    Limit      int
}

type OrderRepository interface {
    Save(ctx context.Context, o *domain.Order) error
    GetByID(ctx context.Context, id string) (*domain.Order, error)
    List(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error)
    UpdateStatus(ctx context.Context, id string, s domain.OrderStatus) error
}

type OrderValidator interface {
    Validate(ctx context.Context, item domain.OrderItem) error
}

type OrderService struct { /* repo + валидаторы, конструируется NewOrderService */ }

func (s *OrderService) CreateOrder(ctx context.Context, customerID string, items []domain.OrderItem) (*domain.Order, error)
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error)
func (s *OrderService) ListOrders(ctx context.Context, f ListFilter) ([]*domain.Order, error)
func (s *OrderService) CancelOrder(ctx context.Context, id string) error
```

### Неделя 5 — HTTP API (`internal/httpapi`)

- `POST /v1/orders` → `201` + заказ; `400` — невалидный запрос; `409` — конфликт.
- `GET /v1/orders/{id}` → `200` / `404`.
- `GET /v1/orders?customer_id=&status=&limit=` → `200`.
- Ошибка — единая схема: `{"error": {"code": "...", "message": "..."}}`.
- Middleware: request ID, логирование, recovery, rate limiting (неделя 11).

### Неделя 6 — хранилище

Миграции `internal/migrations`:
- `0001_create_orders.sql` — таблица `orders` (id text PK, customer_id, status,
  total bigint, created_at timestamptz);
- `0002_create_outbox.sql` — таблица `outbox` (неделя 8): id bigserial PK,
  event_id uuid, event_type text, payload jsonb, created_at, published_at
  timestamptz NULL — признак «ещё не опубликовано».

Реализация `OrderRepository` в `internal/storage/postgres` на pgx/pgxpool.

### Неделя 7 — gRPC (`api/orders/v1/orders.proto`)

```proto
service OrdersService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (stream OrderResponse); // server-streaming
}
```

Второй сервис — `catalog`: `rpc GetProduct(GetProductRequest) returns (Product)`
(`Product`: id, name, price, in_stock). Клиент `internal/client/catalog` обязан
иметь retry с backoff, таймауты и circuit breaker (неделя 7, урок 14).

### Неделя 8 — события

- Топик `orders.created`, ключ партиционирования — `order_id`.
- Полезная нагрузка: `{"order_id", "customer_id", "items": [...], "total",
  "created_at"}`.
- Публикация только через **transactional outbox**: событие пишется в таблицу
  `outbox` в одной транзакции с заказом; фоновое реле публикует и помечает
  `published_at`.
- Консьюмер-сага `payments` обрабатывает событие идемпотентно (дедупликация по
  `order_id`).

## Эволюция по неделям

| Неделя | Что добавляется в итоговый проект |
|---|---|
| 1 | Модуль, layout, Makefile, линтеры; `internal/domain` — модель и валидация заказа |
| 2 | `internal/service` — OrderService поверх интерфейсов; юнит-тесты |
| 3 | Конкурентная валидация позиций; pipeline обработки заказов в service |
| 4 | `internal/cache` — кэш заказов с блокировками; `context` во всех методах |
| 5 | `cmd/orders` + `internal/httpapi` — работающий REST-сервис, graceful shutdown |
| 6 | `internal/storage/postgres` + миграции + интеграционные тесты |
| 7 | `api/orders/v1` proto + `internal/grpcapi`; `internal/client/catalog` (retry, breaker) |
| 8 | `internal/events` — продюсер Kafka + outbox-реле; консьюмер payments |
| 9 | Оптимизация горячего пути; бенчмарки и профилирование сервиса |
| 10 | `internal/telemetry` — slog, Prometheus-метрики, OTel-трейсинг |
| 11 | `internal/ratelimit` + middleware; кэш на Redis + singleflight |
| 12 | Dockerfile, docker-compose всей системы; k8s-манифесты; CI/CD |

## Стандарты итогового проекта

- Ошибки — на английском, без заглавной буквы и точки в конце; оборачиваются через
  `fmt.Errorf("...: %w", err)`; доменные ошибки — sentinel + `errors.Is`.
- Деньги — `int64` копеек, никогда не float.
- Каждый публичный метод сервисного слоя принимает `context.Context` первым аргументом.
- Тесты — table-driven, фейки хранилища — в `internal/...` рядом с пакетом.
- Форматирование — `gofmt`/`gofumpt`, линтер — golangci-lint (конфиг с урока 1).
