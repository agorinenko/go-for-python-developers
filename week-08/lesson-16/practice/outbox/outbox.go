// Package outbox — transactional outbox: фоновое реле публикует события из
// таблицы outbox во внешний транспорт и помечает опубликованные записи.
// Контракт таблицы — CAPSTONE.md, неделя 6: outbox (id bigserial PK,
// event_id uuid, event_type text, payload jsonb, published_at NULL = ещё
// не опубликовано).
package outbox

import (
	"context"
	"encoding/json"
	"time"
)

// Message — строка таблицы outbox. Здесь — минимум, нужный реле;
// в капстоуне добавляются event_id uuid и created_at.
type Message struct {
	ID        int64           // первичный ключ записи outbox
	EventType string          // тип события, например "OrderCreated"
	Payload   json.RawMessage // полезная нагрузка (jsonb)
}

// Store — хранилище outbox-сообщений.
type Store interface {
	// FetchPending возвращает до limit сообщений, ещё не опубликованных
	// (published_at IS NULL), по возрастанию id.
	FetchPending(ctx context.Context, limit int) ([]Message, error)
	// MarkPublished помечает сообщения опубликованными (published_at = now()).
	MarkPublished(ctx context.Context, ids []int64) error
}

// Transport — куда публикует реле (например, Kafka-транспорт урока 15).
type Transport interface {
	Send(ctx context.Context, msg Message) error
}

// fetchBatchSize — сколько сообщений реле берёт за один цикл.
const fetchBatchSize = 100

// Relay — фоновое реле outbox: периодически забирает pending-сообщения
// из Store и публикует их через Transport.
type Relay struct {
	store     Store
	transport Transport
	interval  time.Duration
}

// NewRelay создаёт реле с заданным периодом опроса.
func NewRelay(store Store, transport Transport, interval time.Duration) *Relay {
	return &Relay{store: store, transport: transport, interval: interval}
}

// Run крутит реле до отмены ctx: раз в interval выполняется ProcessOnce.
//
// TODO: реализуй (лекция, раздел «Outbox-реле»):
//  1. time.NewTicker(r.interval), defer ticker.Stop();
//  2. первый цикл допустимо выполнить сразу, дальше — по тикеру;
//  3. select: case <-ctx.Done(): return / case <-ticker.C: ProcessOnce;
//  4. ошибки ProcessOnce НЕ останавливают реле (в капстоуне — в лог,
//     здесь достаточно игнорировать) — недоставленное останется pending
//     и уйдёт в следующем цикле.
func (r *Relay) Run(ctx context.Context) {
	// TODO: реализуй. Заглушка: ждём отмены, ничего не публикуем —
	// тест TestRunPublishesUntilCancel падает на отсутствии публикаций.
	<-ctx.Done()
}

// ProcessOnce — один цикл реле: fetch → send → mark.
// Возвращает число успешно опубликованных сообщений.
//
// TODO: реализуй (важно — частичный прогресс, лекция, раздел «Outbox-реле»):
//  1. store.FetchPending(ctx, fetchBatchSize); ошибка → (0, err);
//  2. по одному transport.Send(ctx, msg);
//  3. при сбое отправки: НЕ бросай всё — пометь УСПЕШНО отправленные id
//     и верни (число успешных, обёрнутая ошибка) — неотправленные останутся
//     pending и уйдут в следующем цикле;
//  4. если отправлены все — MarkPublished всех id; ошибка mark →
//     вернуть (len(msgs), err): события уже доставлены;
//  5. оборачивай ошибки: fmt.Errorf("outbox: send %d: %w", msg.ID, err) —
//     вызывающий код проверяет их через errors.Is.
func (r *Relay) ProcessOnce(ctx context.Context) (int, error) {
	// TODO: реализуй. Заглушка: ничего не публикует.
	return 0, nil
}
