// Package kafkatransport — транспорт событий поверх Kafka (kafka-go).
// В тестах практики не используется — их покрывает фейковый транспорт;
// этот пакет проверяется на реальном брокере (см. README.md, раздел
// «Опробовать с redpanda»).
package kafkatransport

import (
	"context"

	"github.com/segmentio/kafka-go"

	"practice/events"
)

// KafkaTransport отправляет события в Kafka через kafka.Writer.
type KafkaTransport struct {
	writer *kafka.Writer
}

// Проверка на этапе компиляции: KafkaTransport обязан реализовать
// events.Transport.
var _ events.Transport = (*KafkaTransport)(nil)

// NewKafkaTransport создаёт транспорт поверх готового kafka.Writer.
func NewKafkaTransport(writer *kafka.Writer) *KafkaTransport {
	return &KafkaTransport{writer: writer}
}

// Send отправляет событие в топик topic с ключом партиционирования key.
//
// TODO: реализуй отображение на kafka.Writer (лекция, раздел
// «kafka-go: ключевые настройки»):
//  1. собери []kafka.Message из одного сообщения: Topic = topic,
//     Key = []byte(key), Value — сериализация события. Структура Event
//     сериализуется штатным encoding/json (json.Marshal(ev)) —
//     ID, Type и Payload как вложенный объект;
//  2. вызови t.writer.WriteMessages(ctx, msg);
//  3. ошибку верни обёрнутой: fmt.Errorf("kafka send to %s: %w", topic, err).
func (t *KafkaTransport) Send(ctx context.Context, topic, key string, ev events.Event) error {
	// TODO: реализуй. Заглушка ничего не отправляет.
	return nil
}
