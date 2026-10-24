package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"practice/domain"
	"practice/events"
)

// call — один зафиксированный вызов транспорта.
type call struct {
	topic string
	key   string
	ev    events.Event
}

// fakeTransport — фейковый транспорт: записывает вызовы, опционально
// возвращает ошибку. Работает без брокера.
type fakeTransport struct {
	mu    sync.Mutex
	calls []call
	err   error // если не nil — Send возвращает её
}

func (f *fakeTransport) Send(_ context.Context, topic, key string, ev events.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{topic: topic, key: key, ev: ev})
	return f.err
}

func (f *fakeTransport) snapshot() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]call(nil), f.calls...)
}

// payload — структура JSON-нагрузки события по контракту CAPSTONE.md,
// неделя 8: {"order_id", "customer_id", "items", "total", "created_at"}.
type payload struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Items      []struct {
		ProductID string `json:"product_id"`
		Name      string `json:"name"`
		Quantity  int    `json:"quantity"`
		Price     int64  `json:"price"`
	} `json:"items"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
}

// uuidV4 — формат uuid v4: 8-4-4-4-12, версия 4, вариант 8/9/a/b.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func sampleOrder() *domain.Order {
	return domain.NewOrder("o-1", "c-1", []domain.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
		{ProductID: "p-2", Name: "Ручка", Quantity: 5, Price: 50},
	})
}

func TestPublishOrderCreated(t *testing.T) {
	tr := &fakeTransport{}
	p := events.NewPublisher(tr)
	o := sampleOrder()

	if err := p.PublishOrderCreated(context.Background(), o); err != nil {
		t.Fatalf("PublishOrderCreated: unexpected error: %v", err)
	}

	calls := tr.snapshot()
	if len(calls) != 1 {
		t.Fatalf("транспорт получил %d вызовов, want 1", len(calls))
	}
	c := calls[0]

	if c.topic != events.TopicOrderCreated {
		t.Errorf("topic = %q, want %q", c.topic, events.TopicOrderCreated)
	}
	if c.key != o.ID {
		t.Errorf("key = %q, want %q (ключ партиционирования — order_id)", c.key, o.ID)
	}
	if c.ev.Type != events.EventTypeOrderCreated {
		t.Errorf("event type = %q, want %q", c.ev.Type, events.EventTypeOrderCreated)
	}
	if !uuidV4.MatchString(c.ev.ID) {
		t.Errorf("event id = %q, want uuid v4 (8-4-4-4-12)", c.ev.ID)
	}

	var got payload
	if err := json.Unmarshal(c.ev.Payload, &got); err != nil {
		t.Fatalf("payload — невалидный JSON: %v", err)
	}
	if got.OrderID != o.ID {
		t.Errorf("payload.order_id = %q, want %q", got.OrderID, o.ID)
	}
	if got.CustomerID != o.CustomerID {
		t.Errorf("payload.customer_id = %q, want %q", got.CustomerID, o.CustomerID)
	}
	if got.Total != o.Total {
		t.Errorf("payload.total = %d, want %d", got.Total, o.Total)
	}
	if !got.CreatedAt.Equal(o.CreatedAt) {
		t.Errorf("payload.created_at = %v, want %v", got.CreatedAt, o.CreatedAt)
	}
	if len(got.Items) != len(o.Items) {
		t.Fatalf("payload.items: %d позиций, want %d", len(got.Items), len(o.Items))
	}
	for i := range o.Items {
		if got.Items[i].ProductID != o.Items[i].ProductID ||
			got.Items[i].Name != o.Items[i].Name ||
			got.Items[i].Quantity != o.Items[i].Quantity ||
			got.Items[i].Price != o.Items[i].Price {
			t.Errorf("payload.items[%d] = %+v, want %+v", i, got.Items[i], o.Items[i])
		}
	}
}

func TestPublishOrderCreatedUniqueIDs(t *testing.T) {
	tr := &fakeTransport{}
	p := events.NewPublisher(tr)

	for i := 0; i < 2; i++ {
		if err := p.PublishOrderCreated(context.Background(), sampleOrder()); err != nil {
			t.Fatalf("PublishOrderCreated: unexpected error: %v", err)
		}
	}
	calls := tr.snapshot()
	if len(calls) != 2 {
		t.Fatalf("транспорт получил %d вызовов, want 2", len(calls))
	}
	if calls[0].ev.ID == calls[1].ev.ID {
		t.Errorf("ID событий совпадают: %q", calls[0].ev.ID)
	}
}

func TestPublishOrderCreatedTransportError(t *testing.T) {
	sendErr := errors.New("kafka: broker is down")
	tr := &fakeTransport{err: sendErr}
	p := events.NewPublisher(tr)

	err := p.PublishOrderCreated(context.Background(), sampleOrder())
	if !errors.Is(err, sendErr) {
		t.Fatalf("error = %v, want errors.Is(..., %v) — ошибка транспорта должна дойти до вызывающего", err, sendErr)
	}
}
