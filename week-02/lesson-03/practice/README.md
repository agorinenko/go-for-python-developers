# Практика. Урок 3: сервисный слой поверх интерфейсов

Пакет `service` — бизнес-логика заказов, зависящая только от интерфейсов
(`domain.OrderRepository`, `domain.OrderValidator`). Контракт заморожен в
[CAPSTONE.md](../../../CAPSTONE.md), раздел «Неделя 2»: имена типов, интерфейсов
и сигнатуры методов менять нельзя.

Пакет `domain` — минимальная копия доменной модели недели 1 (типы, статусы,
sentinel-ошибки, интерфейсы, `ApplyStatus`). Код ГОТОВЫЙ, менять не нужно:
в капстоуне его заменит настоящий `orders/internal/domain`.

## Задание

Реализуйте `TODO` в `service/service.go` (лекция: интерфейсы на стороне
потребителя, ошибки через `errors.Is`):

1. `CreateOrder` — валидация аргументов, прогон каждой позиции через
   `OrderValidator` (с пробросом `ctx`), генерация ID через `newID()`,
   расчёт `Total`, сохранение через `OrderRepository.Save`.
2. `GetOrder` — делегирование репозиторию; неизвестный ID —
   `domain.ErrOrderNotFound`.
3. `ListOrders` — делегирование репозиторию с фильтром `domain.ListFilter`.
4. `CancelOrder` — получить заказ, применить переход статуса через
   `ApplyStatus(StatusCancelled)`, сохранить через `UpdateStatus`.

Требования к ошибкам:

- пустой `customerID` → `domain.ErrEmptyCustomer`; пустые `items` →
  `domain.ErrNoItems`;
- ошибка валидатора оборачивается с деталями (какая позиция) так, чтобы
  `errors.Is` находил и `domain.ErrInvalidItem`, и исходную ошибку:
  `fmt.Errorf("%w: product %q: %w", domain.ErrInvalidItem, item.ProductID, err)`
  — в `fmt.Errorf` допустимо несколько `%w` (Go 1.20+);
- невалидный переход статуса — `domain.ErrInvalidTransition` (его вернёт
  `ApplyStatus`).

Фейки (`fakeRepo`, `fakeValidator`) уже лежат в `service/service_test.go` —
изучите их: это «рабочие» реализации интерфейсов, именно поэтому сервис не
знает, с чем работает.

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в капстоун

1. В `orders/internal/domain/` капстоуна добавьте из практики
   `domain.ListFilter`, интерфейсы `OrderRepository` и `OrderValidator`
   (контракт недели 2 из CAPSTONE.md — теперь они «заморожены»).
2. Создайте `orders/internal/service/service.go`: скопируйте
   `service/service.go` из практики, замените импорт `practice/domain` на
   `orders/internal/domain`.
3. Скопируйте `service/service_test.go` в `orders/internal/service/`,
   исправив импорты. Фейки остаются в тестовом файле.
4. Добейтесь `go test ./...` в капстоуне.
