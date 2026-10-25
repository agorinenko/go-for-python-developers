# Практика. Урок 10: JSON-контракты, DTO, валидация, ошибки API

Пакет `api` — DTO и (де)сериализация REST API заказов. Контракт
заморожен в [CAPSTONE.md](../../../CAPSTONE.md), раздел «Неделя 5»:

- `POST /v1/orders` → `201` + заказ; `400` — невалидный запрос; `409` — конфликт;
- `GET /v1/orders/{id}` → `200` / `404`;
- единая схема ошибок: `{"error": {"code": "...", "message": "..."}}`.

`domain` — готовая учебная копия доменной модели итогового проекта (её код —
неделя 1, менять не нужно). Всё на стандартной библиотеке.

## Задание

Реализуйте `TODO` в `api/`:

1. **`DecodeCreateOrder`** (`decode.go`) — потоковый декодинг через
   `json.Decoder`:
   - `DisallowUnknownFields` — неизвестные поля отклоняются;
   - ровно одно JSON-значение в потоке (второй `Decode` обязан вернуть
     `io.EOF`);
   - валидация: `customer_id` непуст, `items` непусты, `quantity > 0`,
     `price > 0` — ошибки с внятными текстами, обёрнутые в доменные
     sentinel через `%w` (иначе `WriteError` не отдаст 400).
2. **`ToDomain`** (`decode.go`) — маппинг запроса в `domain.Order`:
   статус `StatusNew`, `Total` по позициям, `ID`/`CreatedAt` не трогаем
   (их проставит сервисный слой). Невалидные данные — доменная ошибка,
   не паника.
3. **`EncodeOrder`** (`encode.go`) — сериализация `*domain.Order` в
   формат `OrderResponse` через `json.NewEncoder`.
4. **`WriteError`** (`error.go`) — маппинг доменных ошибок в HTTP-статусы:
   `ErrOrderNotFound` → `404`, `ErrInvalidItem`/`ErrEmptyCustomer`/
   `ErrNoItems` → `400`, `ErrInvalidTransition` → `409`, остальное →
   `500` с generic-сообщением `internal error` (детали — только для 4xx),
   `Content-Type: application/json`, тело в единой схеме.

## Критерий сдачи

```sh
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в итоговый проект

1. Скопируйте `api/` в `orders/internal/httpapi/` итогового проекта (пакет
   остаётся `api`, или влейте типы в `httpapi` — на ваше усмотрение),
   заменив импорт `practice/domain` → `orders/internal/domain`.
2. Замените обработчики-заглушки урока 9 на настоящие:
   - `handleCreateOrder`: `DecodeCreateOrder(r.Body)` → `req.ToDomain()`
     → `svc.CreateOrder` → `201` + `EncodeOrder`; ошибки — через
     `WriteError` (валидация уйдёт в 400, конфликт — в 409 автоматически);
   - `handleGetOrder`: `chi.URLParam(r, "id")` → `svc.GetOrder` →
     `200` + `EncodeOrder` / `WriteError` (404);
   - `handleListOrders`: параметры `customer_id`, `status`, `limit` →
     `svc.ListOrders` → `200` + массив заказов.
3. Добейтесь `go test ./... && go vet ./...` в итоговом проекте и проверьте
   ошибки вручную: `curl -i localhost:8080/v1/orders` с невалидным телом
   должен вернуть `400` со схемой `{"error": {...}}`.
