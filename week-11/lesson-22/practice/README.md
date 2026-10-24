# Практика. Урок 22: cache-aside, singleflight, инвалидация

Пакет `cache` — будущий `orders/internal/cache` капстоуна. Контракт
(интерфейс `Backend`, сигнатуры `NewCache`, `Get`, `Set`, `Del`) зафиксирован:
менять его нельзя. Поля структуры `Cache` — каркас: локальная карта значений,
мьютекс, `singleflight.Group` — всё уже на месте.

## Задание

Реализуйте `TODO` в `cache/cache.go`:

1. **`Get`** — cache-aside (лекция, разделы «Cache-aside» и «singleflight»):
   - локальный hit (запись с неистёкшим `expiresAt`) — вернуть значение, **не**
     обращаясь к `Backend`;
   - промах — чтение источника, обёрнутое в `c.sf.Do(key, ...)`: параллельные
     `Get` по одному ключу обязаны породить **ровно один** вызов `backend.Get`;
   - источник вернул значение — заполнить кэш: локально
     (`expiresAt = time.Now().Add(c.ttl)`) и через `backend.Set(..., c.ttl)`;
   - ключа нет в источнике — вернуть `(nil, false, nil)`, кэш не заполнять;
   - ошибка источника — вернуть её, кэш не заполнять (ошибки не кэшируются).
2. **`Set`** — прогрев: локально + `backend.Set(ctx, key, value, c.ttl)`.
3. **`Del`** — инвалидация: удалить из локального кэша и вызвать `backend.Del`.

Доступ к локальной карте — под `c.mu` (тесты гоняют `Get` из 100 горутин).
Зависимость `golang.org/x/sync` уже подключена (нужен только `singleflight`).

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное. Ключевые проверки: hit не ходит в источник; 100 параллельных
промахов по одному ключу → 1 вызов источника; TTL из конструктора передаётся
в `Set`; `Del` заставляет следующий `Get` снова читать источник.

## Перенос в капстоун

1. Скопируйте `cache/cache.go` в `orders/internal/cache/` (пакет остаётся
   `cache`; интерфейс `Backend` — контракт для реализаций).
2. Добавьте `github.com/redis/go-redis/v9` в `orders/go.mod` и напишите
   `internal/cache` Redis-реализацию `Backend` (лекция, раздел «Redis»):
   `Get` → `client.Get` (промах — `redis.Nil`, не ошибка), `Set` → `client.Set`
   с TTL, `Del` → `client.Del`; таймауты клиента — из конфига.
3. In-memory кэш недели 4 оставьте как L1: `Cache.Get` сначала смотрит L1,
   затем Redis-бэкенд.
4. В `internal/service` подключите кэш в `GetOrder` (ключ `order:{id}`,
   сериализация JSON); инвалидация — в `UpdateStatus` и `CancelOrder`.
5. Добавьте метрики hit/miss и `shared`-счётчик из singleflight (урок 20);
   в капстоуне: `go test ./...` зелёный.

## Подсказки

- `singleflight.Do` возвращает `(v any, err error, shared bool)` — приведение
  `v` к `[]byte` делайте через type assertion; для «ключ не найден» удобно
  возвращать из функции `nil, nil` и различать на выходе.
- `redis.Nil` из go-redis проверяется через `errors.Is` — в практике роль
  «ключа нет» играет `ok == false` из `backend.Get`.
