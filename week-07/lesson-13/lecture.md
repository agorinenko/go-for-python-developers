# Урок 13. gRPC и protobuf: контракты строже JSON

## Цели урока

- понять proto3: сообщения, сервисы, нумерация полей, совместимость;
- разобраться, что именно генерирует buf/protoc и почему сгенерированный
  код не трогают руками;
- построить gRPC-сервер: unary и server-streaming методы, маппинг
  ошибок в коды статусов;
- оценить, когда gRPC оправдан, а когда это избыточная сложность.

## Задача из итогового проекта

Контракт «Неделя 7» (CAPSTONE.md): `api/orders/v1/orders.proto` с
сервисом `OrdersService` (`CreateOrder`, `GetOrder`, `ListOrders` —
server-streaming) и `internal/grpcapi`. Второй сервис `catalog` с
`GetProduct` — его клиент строим на следующем уроке. HTTP API (неделя 5)
остаётся; gRPC добавляется как второй транспорт для внутренних вызовов.

## proto3: контракт как код

```proto
syntax = "proto3";

package orders.v1;

service OrdersService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (stream OrderResponse);
}

message GetOrderRequest {
  string id = 1;
}

message OrderResponse {
  string id = 1;
  string customer_id = 2;
  repeated Item items = 3;
  string status = 4;
  int64 total = 5; // в копейках
}
```

Что важно понимать про proto3:

- **Номера полей — контракт.** `= 1`, `= 2` — это то, что реально
  передаётся по проводу (теги), а не имена. Имя можно переименовать,
  номер — нет: номер хранится в данных, сгенерированных старыми
  клиентами.
- **Удаление полей запрещено.** Освободившийся номер резервируют
  (`reserved 3;`), чтобы никто не переиспользовал его для другого типа —
  старые клиенты пришлют по нему не то значение.
- **Только добавление.** Новые поля — обязательно с новыми номерами;
  старые серверы игнорируют неизвестные поля (это и есть механизм
  совместимости версий). Это жёстче, чем «add-only JSON»: правила
  проверяются инструментами (buf breaking-change detection), а не
  ревью-привычками.
- **Типы.** Скаляры (`string`, `int32/64`, `bool`, `bytes`, `double`),
  `repeated` (слайс), вложенные `message`, `enum`, `map<K,V>`. Время —
  `google.protobuf.Timestamp` (well-known type), в Go —
  `*timestamppb.Timestamp`. Нет `null`: у скаляров есть zero value, и
  «не прислали» неотличимо от «прислали ноль» — это осознанная цена
  эффективного формата; для nullable-семантики — `optional` или
  `google.protobuf.*Value`.

## Генерация кода: buf/protoc

Из `.proto` генерируется код: сообщения → структуры с методами
сериализации; сервис → интерфейс `OrdersServiceServer`, glue-обработчики
(распаковка запроса → вызов метода → упаковка ответа), `ServiceDesc`
для регистрации и клиент `OrdersServiceClient`. В практике урока этот
слой заменён рукописным эквивалентом, но правило одно: **сгенерированное
не редактируют**. Правки вносятся в `.proto` и перегенерацией.

buf — современный стандарт: `buf lint` (стиль прото-файлов), `buf
breaking` (детектор ломающих изменений — падает, если вы удалили поле
или сменили тип), `buf generate` (генерация с плагинами). protoc —
нижележащий инструмент, buf заменяет его в повседневной работе.

```sh
buf mod init          # buf.yaml: версия, правила lint/breaking
buf generate          # buf.gen.yaml: плагины (protoc-gen-go, protoc-gen-go-grpc)
```

## gRPC-сервер: unary

```go
srv := grpc.NewServer()
grpcapi.RegisterOrdersServiceServer(srv, grpcapi.NewServer(svc))
// srv.Serve(lis) — как http.Server
```

Реализация метода — обычный Go-код с контекстом:

```go
func (s *Server) GetOrder(ctx context.Context, req *GetOrderRequest) (*GetOrderResponse, error) {
	o, err := s.svc.GetOrder(ctx, req.Id)
	if err != nil {
		return nil, mapOrderError(err)
	}
	return &GetOrderResponse{Order: toOrderResponse(o)}, nil
}
```

Маппинг ошибок — главная новинка. В gRPC нет HTTP-статусов: ошибка —
это пара `(code, message)` из перечисления `codes.*`:

```go
func mapOrderError(err error) error {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return status.Error(codes.NotFound, "order not found")
	case errors.Is(err, domain.ErrInvalidItem),
		errors.Is(err, domain.ErrEmptyCustomer),
		errors.Is(err, domain.ErrNoItems):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
```

`status.FromContextError` — правильный способ вернуть отмену/крайний срок:
код `Canceled`/`DeadlineExceeded` выставляется самим gRPC по факту
отмены. Клиент достаёт код через `status.Code(err)` — не сравнивайте
строки и не полагайтесь на `err.Error()`.

## Server-streaming

`ListOrders` отдаёт много заказов — это поток:

```go
func (s *Server) ListOrders(req *ListOrdersRequest, stream OrdersService_ListOrdersServer) error {
	ctx := stream.Context()
	orders, err := s.svc.ListOrders(ctx, f)
	if err != nil {
		return mapOrderError(err)
	}
	for i := 0; i < len(orders); i += chunkSize {
		if err := ctx.Err(); err != nil { // клиент ушёл — прекращаем
			return status.FromContextError(err).Err()
		}
		for _, o := range orders[i:end] {
			if err := stream.Send(toOrderResponse(o)); err != nil {
				return err
			}
		}
	}
	return nil // поток закрывается корректно, клиент получит io.EOF
}
```

Две обязанности сервера: **проверять контекст между Send** (иначе шлём
в никуда после отмены) и **возвращать nil в конце** (тогда клиентский
`Recv` завершается `io.EOF` — нормальное завершение, не ошибка). Чанки
по 2 в практике — учебный акцент; в проде размер подбирают по лимитам
сообщений и бэкпрешшуру.

## Почему gRPC «быстрее и строже» JSON-REST

- **Бинарный формат** (protobuf wire format): компактнее JSON в разы,
  без парсинга текста — для горячих внутренних вызовов это заметно.
- **Схема = контракт**: типы проверены компилятором на обеих сторонах.
  В JSON контракт — это «OpenAPI, если повезло»; в gRPC он физически не
  может разойтись с кодом.
- **HTTP/2 из коробки**: мультиплексирование (десятки потоков на одном
  соединении), крайние сроки, отмена — всё это штатный транспорт, а не
  надстройка.
- **Streaming** — first-class: client-, server-, bidirectional.

Когда gRPC **избыточен**: публичный API для сторонних клиентов (им нужен
curl и браузер — JSON/REST проще), браузерный фронтенд (нужен grpc-web
прокси), команда без опыта protobuf-тулинга. Классическая раскладка:
внешний REST (неделя 5) + внутренний gRPC между сервисами (orders →
catalog) — ровно как в итоговом проекте.

## Go vs Python: grpcio и aiohttp

```python
# grpc.aio: асинхронный gRPC-сервер
class OrdersServicer(orders_pb2_grpc.OrdersServiceServicer):
    async def GetOrder(self, request, context):
        order = await self.svc.get_order(request.id)
        return orders_pb2.GetOrderResponse(order=...)
```

Python-стек тот же по форме: `.proto` → генерация (`grpcio-tools`),
сервис-класс с методами, `grpc.aio` для asyncio. Различия:

- В Go `stream.Send` — блокирующий вызов в горутине; сервер сам управляет
  параллелизмом. В Python важно не заснуть в event loop — асинхронные
  методы обязательны, иначе весь сервис встанет.
- Коды статусов те же (`grpc.StatusCode.NOT_FOUND`) — протокол общий,
  меняется только среда выполнения.
- Альтернатива «Python: aiohttp + JSON» — это REST-вариант; контракт
  типов придётся нести отдельно (pydantic + ручная дисциплина), тогда
  как в gRPC он — из proto.

**C#**: gRPC на ASP.NET Core + protobuf — ближайший аналог по духу
(строгая типизация, генерация из proto); в .NET это часть экосистемы,
а не отдельный сервер.

## Подводные камни и анти-паттерны

1. **Сравнение `err.Error()` вместо `status.Code`.** Код статуса — это
   контракт; текст может меняться.
2. **Забытый `status.FromContextError`.** Вернув `context.Canceled`
   напрямую, получите `codes.Unknown` у клиента — клиент не отличит
   отмену от внутренней ошибки.
3. **Не проверять `ctx.Err()` в стриме.** После отмены клиента сервер
   продолжает слать в закрытый поток — впустую тратит ресурсы.
4. **Редактирование сгенерированного кода.** Пропадёт при следующей
   генерации. Правьте `.proto`.
5. **Переиспользование номеров полей.** Удалённое поле `= 3` с новым
   смыслом — старые клиенты будут присылать по нему старый тип.
   `reserved` и `buf breaking` — страховка.
6. **Возвращать `nil` как «ок» из стрима** — правильно; возвращать
   ошибку, чтобы «просто закрыть поток», — нет: клиент получит код
   ошибки вместо `io.EOF`.
7. **Дедлайны не заданы.** gRPC не выставляет крайние сроки сам: без
   `context.WithTimeout` на клиенте серверный вызов может висеть вечно.
   Подробно — урок 14.
8. **`repeated` для больших списков в unary.** Десятки тысяч элементов в
   одном сообщении — память обеих сторон; для этого и существует
   streaming.
9. **Reflection в проде.** `grpc/reflection/v1` удобен для отладки
   (grpcurl), но раскрывает схему API наружу — включайте осознанно.
10. **Забыть `buf breaking` в CI.** Контракт «защищён» только пока его
    ломающие изменения кто-то ловит автоматически.

## Что делаем в итоговом проекте

1. `orders/api/orders/v1/orders.proto` — контракт из практики; `buf.yaml`
   с `buf lint` + `buf breaking`, генерация в `internal/grpcapi`.
2. `internal/grpcapi`: `Server` с маппингом ошибок и server-streaming
   `ListOrders` — логика из практики поверх сгенерированного кода.
3. `cmd/orders`: `grpc.NewServer()` на отдельном порту (или cmux);
   reflection — только для dev.
4. Второй proto: `api/catalog/v1/catalog.proto` с `GetProduct` — под
   клиент, который строим на уроке 14.

## Материалы для углубления

- [Protocol Buffers: Language Guide (proto3)](https://protobuf.dev/programming-guides/proto3/) — официальный гайд.
- [buf](https://buf.build/docs/) — lint, breaking, generate.
- [grpc-go](https://github.com/grpc/grpc-go) — реализация; [quick start](https://grpc.io/docs/languages/go/quickstart/).
- [gRPC status codes](https://grpc.io/docs/guides/status-codes/) — семантика кодов.
- [timestamppb](https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb) — Timestamp в Go.
- [grpcurl](https://github.com/fullstorydev/grpcurl) — отладка без написания клиента.

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
