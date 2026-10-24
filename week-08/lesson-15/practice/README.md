# Практика. Урок 15: Kafka — событийная шина

Пакеты практики:

- `events` — доменные события и продюсер: тип `Event`, интерфейс `Transport`,
  `Publisher` с методом `PublishOrderCreated`;
- `domain` — готовая минимальная копия доменной модели заказа (каноническая —
  в капстоуне, `orders/internal/domain`);
- `kafkatransport` — реализация `Transport` поверх `kafka-go`.

Контракт события заморожен в [CAPSTONE.md](../../../CAPSTONE.md), раздел
«Неделя 8»: топик `orders.created`, ключ партиционирования — `order_id`,
payload `{"order_id", "customer_id", "items", "total", "created_at"}`.

## Задание

Реализуйте `TODO`:

1. `events.Publisher.PublishOrderCreated` (`events/events.go`):
   - ID события — uuid v4 **через `crypto/rand` вручную** (16 байт, биты
     версии/варианта, формат 8-4-4-4-12), без внешних зависимостей;
   - payload — JSON по контракту (см. выше; `created_at` — RFC 3339,
     штатная сериализация `time.Time`);
   - ключ — `o.ID`, топик — `events.TopicOrderCreated`;
   - ошибка транспорта возвращается вызывающему с обёрткой `%w`.
2. `kafkatransport.KafkaTransport.Send` (`kafkatransport/kafka_transport.go`):
   маппинг на `kafka.Writer.WriteMessages` — `Topic`, `Key`, `Value`
   (сериализация `Event` штатным `encoding/json`).

Подсказки — в лекции: разделы «Сериализация» и «kafka-go: ключевые настройки».

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное. Тесты работают **без брокера** — через фейковый транспорт.

## Опробовать с redpanda (вручную, не обязательно для сдачи)

Поднимите совместимый с Kafka брокер в docker:

```sh
docker run -d --rm --name redpanda -p 9092:9092 \
  docker.redpanda.com/redpanda/redpanda:latest \
  redpanda start --overprovisioned --smp 1 --memory 512M \
  --node-id 0 --check=false --kafka-addr 0.0.0.0:9092 \
  --advertise-kafka-addr 127.0.0.1:9092
```

Пример публикации (временный файл `main.go` в корне практики):

```go
package main

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"

	"practice/domain"
	"practice/events"
	"practice/kafkatransport"
)

func main() {
	writer := &kafka.Writer{
		Addr:     kafka.TCP("127.0.0.1:9092"),
		Balancer: &kafka.Hash{}, // партиция — по ключу
	}
	defer writer.Close()

	p := events.NewPublisher(kafkatransport.NewKafkaTransport(writer))
	o := domain.NewOrder("o-1", "c-1", []domain.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
	})
	if err := p.PublishOrderCreated(context.Background(), o); err != nil {
		panic(err)
	}
	fmt.Println("published")
}
```

Прочитайте сообщение консольным консьюмером:

```sh
docker exec redpanda rpk topic consume orders.created \
  --brokers 127.0.0.1:9092 --offset start
```

Проверьте: ключ сообщения — `o-1`, партиция — та, которую выбрал `Hash`
по ключу; два сообщения с одинаковым `order_id` уходят в одну партицию —
порядок обработки в рамках ключа сохранён.

## Перенос в капстоун

1. Скопируйте `events/events.go` в `orders/internal/events/events.go`,
   переименуйте пакет в `events`, замените импорт `practice/domain` на
   `orders/internal/domain` (копию `domain/` из практики **не** переносите —
   в капстоуне домен уже есть).
2. Скопируйте `kafkatransport/kafka_transport.go` в
   `orders/internal/events/kafka_transport.go` (пакет остаётся `events` —
   отдельный пакет не нужен).
3. Добавьте `github.com/segmentio/kafka-go` в `orders/go.mod`.
4. В `cmd/orders/main.go` создайте `kafka.Writer` (адрес из конфига,
   `Balancer: &kafka.Hash{}`), соберите `Publisher` и передайте его в сервисный
   слой: вызывайте `PublishOrderCreated` после успешного сохранения заказа.
5. Запустите тесты капстоуна: `go test ./...`.

Обратите внимание: на следующем уроке публикация «в лоб» будет заменена на
transactional outbox (CAPSTONE.md, неделя 8 — публикация только через таблицу
`outbox`). `Transport` и сериализация события останутся — их переиспользует
outbox-реле.
