# Практика. Урок 9: HTTP-сервер, роутер, middleware, graceful shutdown

Пакет `httpapi` — транспортный слой сервиса заказов. Контракт маршрутов
заморожен в [CAPSTONE.md](../../../CAPSTONE.md), раздел «Неделя 5»:

- `POST /v1/orders` → `201`;
- `GET /v1/orders/{id}` → `200`/`404`;
- `GET /v1/orders` → `200`;
- единая схема ошибок `{"error": {...}}` появится в уроке 10.

Пакеты `domain` и `service` — готовые учебные копии капстоуна (in-memory
хранилище), они дают тестам реальный бэкенд. Хэндлеры — осознанные
заглушки: полный контракт (JSON, DTO, коды ошибок) делается в уроке 10.

## Задание

Реализуйте `TODO` в `httpapi/` (всё необходимое — в лекции):

1. **`NewRouter`** (`router.go`) — зарегистрируйте маршруты chi:
   `POST /v1/orders`, `GET /v1/orders/{id}`, `GET /v1/orders`.
2. **`Wrap`** (`middleware.go`) — обёртывание `http.Handler` списком
   middleware: первый в списке — самый внешний слой.
3. **`Recover`** (`middleware.go`) — перехват паники: логирование через
   `slog` и ответ `500` с телом `internal error`.
4. **`Logging`** (`middleware.go`) — структурированный лог запроса через
   обёртку `ResponseWriter`, запоминающую статус.
5. **`NewRouter` — порядок middleware**: соберите цепочку через `Wrap`
   в порядке `RequestID → Recover → Logging` (RequestID — внешний слой:
   заголовок `X-Request-ID` должен быть в ответе даже при панике хэндлера).
6. **`Run`** (`run.go`) — сервер с таймаутами (`ReadHeaderTimeout`,
   `ReadTimeout`, `WriteTimeout`, `IdleTimeout` — дефолты опасны, см. лекцию),
   запуск `ListenAndServe` в горутине и graceful shutdown: отмена `ctx` →
   `Shutdown` с таймаутом 10s, `http.ErrServerClosed` — не ошибка.

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

`go vet` молчит, `go test` зелёный.

## Перенос в капстоун

По контракту [CAPSTONE.md](../../../CAPSTONE.md), неделя 5 — сервис должен
запускаться:

1. Скопируйте `httpapi/` в `orders/internal/httpapi/` капстоуна, заменив
   импорты: `practice/domain` → `orders/internal/domain`,
   `practice/service` → `orders/internal/service`.
   Интерфейс `httpapi.Service` в капстоуне реализует
   `*service.OrderService` — ничего менять не нужно.
2. Напишите `orders/cmd/orders/main.go`: `slog`-логгер, `httpapi.NewRouter`,
   контекст через `signal.NotifyContext(context.Background(), os.Interrupt,
   syscall.SIGTERM)` и запуск `httpapi.Run(ctx, addr, router)`.
   Адрес читайте из `PORT`/`ORDERS_ADDR` с дефолтом `:8080`.
3. Добейтесь `go test ./... && go vet ./...` внутри капстоуна и ручного
   smoke-теста: `curl -i localhost:8080/v1/orders`.
