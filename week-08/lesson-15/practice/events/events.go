// Package events — публикация доменных событий сервиса заказов.
// Контракт из CAPSTONE.md (неделя 8): топик orders.created,
// ключ партиционирования — order_id.
package events

import (
	"context"
	"encoding/json"

	"practice/domain"
)

// EventTypeOrderCreated — тип события «заказ создан».
const EventTypeOrderCreated = "OrderCreated"

// TopicOrderCreated — топик Kafka для событий создания заказа.
const TopicOrderCreated = "orders.created"

// Event — событие, публикуемое в шину.
type Event struct {
	ID      string          // уникальный идентификатор события (uuid)
	Type    string          // тип события, например EventTypeOrderCreated
	Payload json.RawMessage // полезная нагрузка — уже сериализованный JSON
}

// Transport — транспорт доставки событий (в капстоуне — Kafka).
type Transport interface {
	Send(ctx context.Context, topic, key string, ev Event) error
}

// Publisher публикует доменные события через Transport.
type Publisher struct {
	t Transport
}

// NewPublisher создаёт Publisher поверх заданного транспорта.
func NewPublisher(t Transport) *Publisher {
	return &Publisher{t: t}
}

// PublishOrderCreated публикует событие создания заказа.
//
// TODO: реализуй (лекция, разделы «Сериализация» и «Продюсер»):
//  1. сгенерируй уникальный ID события — uuid v4 — через crypto/rand
//     вручную, без внешних зависимостей: 16 случайных байт, биты версии
//     и варианта, формат 8-4-4-4-12 строчными hex;
//  2. собери payload по контракту CAPSTONE.md (неделя 8):
//     {"order_id", "customer_id", "items", "total", "created_at"},
//     items — массив {"product_id", "name", "quantity", "price"};
//  3. отправь событие: topic = TopicOrderCreated, key = o.ID
//     (ключ партиционирования — order_id), Type = EventTypeOrderCreated;
//  4. ошибку транспорта верни вызывающему коду с обёрткой %w.
func (p *Publisher) PublishOrderCreated(ctx context.Context, o *domain.Order) error {
	// TODO: реализуй. Заглушка ничего не публикует — тесты на это и падают.
	return nil
}
