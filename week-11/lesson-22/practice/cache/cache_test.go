package cache_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"practice/cache"
)

// fakeBackend — источник данных со счётчиками вызовов.
// origin — данные «источника» (в капстоуне — PostgreSQL);
// stored — то, что записано в кэш-хранилище через Set (в капстоуне — Redis).
type fakeBackend struct {
	mu       sync.Mutex
	origin   map[string][]byte
	stored   map[string]storedEntry
	getCalls int
	lastTTL  time.Duration
	getErr   error
}

type storedEntry struct {
	value []byte
	ttl   time.Duration
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		origin: map[string][]byte{},
		stored: map[string]storedEntry{},
	}
}

func (f *fakeBackend) Get(ctx context.Context, key string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	v, ok := f.origin[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), v...), true, nil
}

func (f *fakeBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stored[key] = storedEntry{value: append([]byte(nil), value...), ttl: ttl}
	f.lastTTL = ttl
	return nil
}

func (f *fakeBackend) Del(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.stored, key)
	return nil
}

func (f *fakeBackend) snapshot() (calls int, stored map[string]storedEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCalls, f.stored
}

const orderJSON = `{"id":"o-1","customer_id":"c-1","status":"new"}`

func TestGetMissFillsCache(t *testing.T) {
	fb := newFakeBackend()
	fb.origin["o-1"] = []byte(orderJSON)
	ttl := 30 * time.Second
	c := cache.NewCache(fb, ttl)

	v, ok, err := c.Get(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: ключ не найден после чтения источника")
	}
	if string(v) != orderJSON {
		t.Fatalf("Get value = %q, want %q", v, orderJSON)
	}

	calls, stored := fb.snapshot()
	if calls != 1 {
		t.Fatalf("backend.Get calls = %d, want 1 (промах → ровно один вызов источника)", calls)
	}
	se, inStore := stored["o-1"]
	if !inStore {
		t.Fatal("промах не заполнил кэш: backend.Set не вызван")
	}
	if string(se.value) != orderJSON {
		t.Fatalf("в кэш записано %q, want %q", se.value, orderJSON)
	}
	if se.ttl != ttl {
		t.Fatalf("backend.Set получил ttl = %v, want %v", se.ttl, ttl)
	}
}

func TestGetHitSkipsBackend(t *testing.T) {
	fb := newFakeBackend()
	fb.origin["o-1"] = []byte(orderJSON)
	c := cache.NewCache(fb, time.Minute)

	if _, _, err := c.Get(context.Background(), "o-1"); err != nil {
		t.Fatalf("первый Get: %v", err)
	}
	v, ok, err := c.Get(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("второй Get: %v", err)
	}
	if !ok || string(v) != orderJSON {
		t.Fatalf("hit: got (%q, %v), want (%q, true)", v, ok, orderJSON)
	}

	calls, _ := fb.snapshot()
	if calls != 1 {
		t.Fatalf("backend.Get calls = %d, want 1: hit не должен ходить в источник", calls)
	}
}

func TestGetSingleflightDedup(t *testing.T) {
	fb := newFakeBackend()
	fb.origin["o-1"] = []byte(orderJSON)
	c := cache.NewCache(fb, time.Minute)

	const workers = 100
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, ok, err := c.Get(context.Background(), "o-1")
			if err != nil {
				errs <- fmt.Errorf("Get: %w", err)
				return
			}
			if !ok {
				errs <- fmt.Errorf("Get: ключ o-1 не найден")
				return
			}
			if string(v) != orderJSON {
				errs <- fmt.Errorf("Get value = %q, want %q", v, orderJSON)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	calls, _ := fb.snapshot()
	if calls != 1 {
		t.Fatalf("backend.Get calls = %d, want 1: singleflight должен дедуплицировать параллельные промахи", calls)
	}
}

func TestSetPassesTTL(t *testing.T) {
	fb := newFakeBackend()
	ttl := 5 * time.Minute
	c := cache.NewCache(fb, ttl)

	if err := c.Set(context.Background(), "o-1", []byte(orderJSON)); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, stored := fb.snapshot()
	se, inStore := stored["o-1"]
	if !inStore {
		t.Fatal("Set: значение не попало в кэш-хранилище")
	}
	if string(se.value) != orderJSON {
		t.Fatalf("Set value = %q, want %q", se.value, orderJSON)
	}
	if se.ttl != ttl {
		t.Fatalf("backend.Set получил ttl = %v, want %v", se.ttl, ttl)
	}
}

func TestDelInvalidates(t *testing.T) {
	fb := newFakeBackend()
	fb.origin["o-1"] = []byte(orderJSON)
	c := cache.NewCache(fb, time.Minute)

	if _, _, err := c.Get(context.Background(), "o-1"); err != nil {
		t.Fatalf("первый Get: %v", err)
	}
	if err := c.Del(context.Background(), "o-1"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	_, stored := fb.snapshot()
	if _, inStore := stored["o-1"]; inStore {
		t.Fatal("после Del значение осталось в кэш-хранилище")
	}

	v, ok, err := c.Get(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("Get после Del: %v", err)
	}
	if !ok || string(v) != orderJSON {
		t.Fatalf("Get после Del: got (%q, %v), want (%q, true) — инвалидированный ключ читается заново", v, ok, orderJSON)
	}

	calls, _ := fb.snapshot()
	if calls != 2 {
		t.Fatalf("backend.Get calls = %d, want 2: после Del следующий Get снова идёт в источник", calls)
	}
}

func TestGetPropagatesError(t *testing.T) {
	fb := newFakeBackend()
	wantErr := errors.New("backend is down")
	fb.getErr = wantErr
	c := cache.NewCache(fb, time.Minute)

	_, _, err := c.Get(context.Background(), "o-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Get error = %v, want errors.Is %v", err, wantErr)
	}

	_, stored := fb.snapshot()
	if _, inStore := stored["o-1"]; inStore {
		t.Fatal("ошибку источника нельзя кэшировать: backend.Set вызван")
	}
}

func TestGetMissReturnsNotFound(t *testing.T) {
	fb := newFakeBackend()
	c := cache.NewCache(fb, time.Minute)

	v, ok, err := c.Get(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("Get: неожиданный hit для отсутствующего ключа")
	}
	if v != nil {
		t.Fatalf("Get value = %q, want nil", v)
	}

	_, stored := fb.snapshot()
	if _, inStore := stored["o-1"]; inStore {
		t.Fatal("промах не должен записывать в кэш пустое значение")
	}
}
