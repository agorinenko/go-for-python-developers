package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"practice/outbox"
)

var (
	errSend  = errors.New("send failed")
	errFetch = errors.New("fetch failed")
	errMark  = errors.New("mark failed")
)

// fakeStore — хранилище в памяти: pending-список и журнал MarkPublished.
type fakeStore struct {
	mu       sync.Mutex
	pending  []outbox.Message
	marked   []int64
	fetchErr error
	markErr  error
}

func (s *fakeStore) FetchPending(_ context.Context, limit int) ([]outbox.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	if limit > len(s.pending) {
		limit = len(s.pending)
	}
	return append([]outbox.Message(nil), s.pending[:limit]...), nil
}

func (s *fakeStore) MarkPublished(_ context.Context, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markErr != nil {
		return s.markErr
	}
	s.marked = append(s.marked, ids...)
	byID := make(map[int64]bool, len(ids))
	for _, id := range ids {
		byID[id] = true
	}
	var rest []outbox.Message
	for _, m := range s.pending {
		if !byID[m.ID] {
			rest = append(rest, m)
		}
	}
	s.pending = rest
	return nil
}

func (s *fakeStore) snapshot() (marked []int64, pending []outbox.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.marked...), append([]outbox.Message(nil), s.pending...)
}

// fakeTransport — транспорт: записывает отправленные сообщения, умеет
// падать на заданных id.
type fakeTransport struct {
	mu      sync.Mutex
	sent    []outbox.Message
	failIDs map[int64]bool
}

func (t *fakeTransport) Send(_ context.Context, msg outbox.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failIDs[msg.ID] {
		return fmt.Errorf("%w: message %d", errSend, msg.ID)
	}
	t.sent = append(t.sent, msg)
	return nil
}

func (t *fakeTransport) snapshot() []outbox.Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]outbox.Message(nil), t.sent...)
}

// msgs строит n сообщений с id 1..n.
func msgs(n int) []outbox.Message {
	out := make([]outbox.Message, n)
	for i := range out {
		out[i] = outbox.Message{
			ID:        int64(i + 1),
			EventType: "OrderCreated",
			Payload:   json.RawMessage(fmt.Sprintf(`{"order_id":"o-%d"}`, i+1)),
		}
	}
	return out
}

func TestProcessOncePublishesAll(t *testing.T) {
	store := &fakeStore{pending: msgs(3)}
	tr := &fakeTransport{}
	relay := outbox.NewRelay(store, tr, time.Second)

	n, err := relay.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: unexpected error: %v", err)
	}
	if n != 3 {
		t.Fatalf("опубликовано %d, want 3", n)
	}

	sent := tr.snapshot()
	if len(sent) != 3 {
		t.Fatalf("транспорт получил %d сообщений, want 3", len(sent))
	}
	for i, want := range []int64{1, 2, 3} {
		if sent[i].ID != want {
			t.Errorf("sent[%d].ID = %d, want %d (порядок по возрастанию id)", i, sent[i].ID, want)
		}
	}
	marked, pending := store.snapshot()
	if len(marked) != 3 || marked[0] != 1 || marked[1] != 2 || marked[2] != 3 {
		t.Errorf("marked = %v, want [1 2 3]", marked)
	}
	if len(pending) != 0 {
		t.Errorf("pending осталось %d сообщений, want 0", len(pending))
	}
}

func TestProcessOncePartialFailure(t *testing.T) {
	store := &fakeStore{pending: msgs(3)}
	tr := &fakeTransport{failIDs: map[int64]bool{2: true}}
	relay := outbox.NewRelay(store, tr, time.Second)

	n, err := relay.ProcessOnce(context.Background())
	if err == nil {
		t.Fatal("ProcessOnce: ожидалась ошибка при сбое отправки, got nil")
	}
	if !errors.Is(err, errSend) {
		t.Fatalf("error = %v, want errors.Is(..., errSend)", err)
	}
	if n != 1 {
		t.Fatalf("число успешных = %d, want 1", n)
	}

	sent := tr.snapshot()
	if len(sent) != 1 || sent[0].ID != 1 {
		t.Errorf("sent = %v, want только сообщение 1", sent)
	}
	marked, pending := store.snapshot()
	if len(marked) != 1 || marked[0] != 1 {
		t.Errorf("marked = %v, want [1] — успешно отправленное помечается даже при сбое", marked)
	}
	if len(pending) != 2 || pending[0].ID != 2 || pending[1].ID != 3 {
		t.Errorf("pending = %v, want [2 3] — неотправленные остаются для следующего цикла", pending)
	}
}

func TestProcessOnceFetchError(t *testing.T) {
	store := &fakeStore{pending: msgs(2), fetchErr: errFetch}
	tr := &fakeTransport{}
	relay := outbox.NewRelay(store, tr, time.Second)

	n, err := relay.ProcessOnce(context.Background())
	if !errors.Is(err, errFetch) {
		t.Fatalf("error = %v, want errors.Is(..., errFetch)", err)
	}
	if n != 0 {
		t.Fatalf("число успешных = %d, want 0", n)
	}
	if got := len(tr.snapshot()); got != 0 {
		t.Errorf("транспорт получил %d сообщений при сбое fetch, want 0", got)
	}
}

func TestProcessOnceMarkError(t *testing.T) {
	store := &fakeStore{pending: msgs(3), markErr: errMark}
	tr := &fakeTransport{}
	relay := outbox.NewRelay(store, tr, time.Second)

	n, err := relay.ProcessOnce(context.Background())
	if !errors.Is(err, errMark) {
		t.Fatalf("error = %v, want errors.Is(..., errMark)", err)
	}
	if n != 3 {
		t.Fatalf("число успешных = %d, want 3 — события уже доставлены", n)
	}
	if got := len(tr.snapshot()); got != 3 {
		t.Errorf("транспорт получил %d сообщений, want 3", got)
	}
}

func TestRunPublishesUntilCancel(t *testing.T) {
	store := &fakeStore{pending: msgs(2)}
	tr := &fakeTransport{}
	relay := outbox.NewRelay(store, tr, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan struct{})
	go func() {
		defer close(done)
		relay.Run(ctx)
	}()

	// ждём, пока оба сообщения будут опубликованы (полинг с дедлайном)
	deadline := time.Now().Add(time.Second)
	for {
		if len(tr.snapshot()) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("реле не опубликовало сообщения за 1 с — цикл Run не работает")
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
		// быстрый выход после отмены — ок
	case <-time.After(time.Second):
		t.Fatal("Run не завершился в течение 1 с после отмены ctx")
	}
}
