# Практика. Урок 13: gRPC и protobuf

Контракт gRPC заказан в [CAPSTONE.md](../../../CAPSTONE.md) (неделя 7):
`OrdersService` с `CreateOrder`, `GetOrder` и `ListOrders`
(server-streaming). Источник истины контракта — `orders.proto` в корне
практики.

**Честная оговорка**: в реальном проекте Go-код сервера **генерируется**
из `orders.proto` через buf/protoc — вручную интерфейсы `grpc.ServiceDesc`
никто не пишет. В этой практике генерация заменена рукописным
эквивалентом в `grpcapi/messages.go`, `grpcapi/service.go`,
`grpcapi/client.go` — чтобы модуль работал без установленного protoc.
Рукописные структуры не реализуют `proto.Message`, поэтому для них
используется JSON-кодек gRPC (`grpcapi.Codec`, применяется в тестах
через `grpc.ForceCodec`) — в итоговом проекте с настоящей генерацией кодек не
нужен. Структура сообщений поле-в-поле соответствует `orders.proto`.

`domain` и `service` — готовые учебные копии итогового проекта.

## Задание

Реализуйте `TODO` в `grpcapi/server.go`:

1. **`CreateOrder`** — валидация запроса (`customer_id` непуст, `items`
   непусты, `quantity > 0`, `price > 0`) → `codes.InvalidArgument` с
   внятным текстом; маппинг позиций `int32 → int`; вызов
   `svc.CreateOrder`; маппинг ошибок: `ErrEmptyCustomer`/`ErrNoItems`/
   `ErrInvalidItem` → `InvalidArgument`, `ErrInvalidTransition` →
   `AlreadyExists`, `ErrOrderNotFound` → `NotFound`, `context.Canceled`/
   `DeadlineExceeded` → `status.FromContextError`, остальное →
   `Internal`; успех — `CreateOrderResponse{Order: toOrderResponse(o)}`.
2. **`GetOrder`** — `svc.GetOrder(ctx, req.Id)`, тот же маппинг ошибок.
3. **`ListOrders`** — server-streaming: контекст стрима
   (`stream.Context()`), маппинг фильтра, вызов `svc.ListOrders`,
   отправка заказов чанками по 2 с проверкой `ctx.Err()` между Send;
   отмена → `status.FromContextError(ctx.Err()).Err()`.

`orders.proto` в итоговом проекте не трогайте — он фиксирует контракт.

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в итоговый проект

1. Положите `orders.proto` в `orders/api/orders/v1/orders.proto` итогового проекта
   (контракт «Неделя 7»).
2. Установите [buf](https://buf.build) и сгенерируйте код:
   `buf mod init && buf generate` с конфигом, который кладёт
   `ordersv1` в `orders/internal/grpcapi` (или временно `protoc`).
   Сгенерированные типы займут место рукописных `messages.go`/
   `service.go`/`client.go`; `toOrderResponse` и логика методов
   `Server` переезжают как есть (замените `CreatedAt string` на
   `timestamppb.Timestamp` при маппинге).
3. Регистрация в `cmd/orders`: `grpc.NewServer()`,
   `grpcapi.RegisterOrdersServiceServer(srv, grpcapi.NewServer(svc))`,
   сервер на `GRPC_ADDR` рядом с HTTP (или через cmux). Второй сервис
   `catalog` с `GetProduct` — по тому же шаблону (клиент к нему — урок 14).
4. Добейтесь `go test ./... && go vet ./...`; тесты из практики
   (bufconn) переносите в пакет `grpcapi` итогового проекта.
