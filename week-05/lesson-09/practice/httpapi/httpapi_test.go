package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"practice/httpapi"
	"practice/service"
)

// invoke запускает хэндлер и превращает панику в чистый fail теста
// (паника не должна ронять тестовый процесс).
func invoke(t *testing.T, h http.Handler, req *http.Request) (rec *httptest.ResponseRecorder) {
	t.Helper()
	rec = httptest.NewRecorder()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panicked: %v", r)
		}
	}()
	h.ServeHTTP(rec, req)
	return rec
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	return httpapi.NewRouter(service.NewOrderService())
}

func TestRoutingRegisteredRoutes(t *testing.T) {
	h := newTestRouter(t)
	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"create order", http.MethodPost, "/v1/orders", http.StatusCreated},
		{"get order by id", http.MethodGet, "/v1/orders/o-1", http.StatusOK},
		{"list orders", http.MethodGet, "/v1/orders", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.want {
				t.Fatalf("%s %s: status = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}

func TestRoutingNotFound(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/orders/o-1/extra", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/orders/o-1/extra: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRoutingMethodNotAllowed(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/orders", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE /v1/orders: status = %d, want %d (chi отвечает 405 на известный путь с неверным методом)", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRequestIDGenerated(t *testing.T) {
	h := httpapi.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("X-Request-ID header is empty")
	}
	if len(id) != 32 {
		t.Fatalf("X-Request-ID = %q, want 32 hex characters", id)
	}
}

func TestRequestIDPreserved(t *testing.T) {
	h := httpapi.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != "client-supplied" {
			t.Errorf("handler saw X-Request-ID = %q, want %q", got, "client-supplied")
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-supplied")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "client-supplied" {
		t.Fatalf("X-Request-ID = %q, want %q", got, "client-supplied")
	}
}

func TestRecoverReturns500(t *testing.T) {
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	h := httpapi.Recover(boom)
	rec := invoke(t, h, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "internal error") {
		t.Fatalf("body = %q, want to contain %q", rec.Body.String(), "internal error")
	}
}

// TestMiddlewareChainOrderAndPanic проверяет цепочку RequestID → Recover:
// заголовок X-Request-ID обязан появиться в ответе, даже когда хэндлер
// паникует, — значит, RequestID стоит снаружи Recover.
func TestMiddlewareChainOrderAndPanic(t *testing.T) {
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	chain := httpapi.Wrap(boom, httpapi.RequestID, httpapi.Recover)
	rec := invoke(t, chain, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID missing: RequestID должен быть снаружи Recover")
	}
}

// TestWrapOrder проверяет семантику Wrap: первый middleware в списке —
// самый внешний (входит первым, выходит последним).
func TestWrapOrder(t *testing.T) {
	var events []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			events = append(events, "first:enter")
			next.ServeHTTP(w, r)
			events = append(events, "first:exit")
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			events = append(events, "second:enter")
			next.ServeHTTP(w, r)
			events = append(events, "second:exit")
		})
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events = append(events, "inner")
	})

	h := httpapi.Wrap(inner, mw1, mw2)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"first:enter", "second:enter", "inner", "second:exit", "first:exit"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := httpapi.Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req.Header.Set("X-Request-ID", "req-123")
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, want := range []string{"http request", "method=GET", "path=/v1/orders", "status=418", "req-123"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log output %q does not contain %q", out, want)
		}
	}
}

// TestRunGracefulShutdown поднимает настоящий сервер на случайном порту,
// убеждается, что он принимает соединения, отменяет контекст и ждёт
// корректной остановки.
func TestRunGracefulShutdown(t *testing.T) {
	// Свободный случайный порт: слушаем, узнаём адрес, закрываем.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- httpapi.Run(ctx, addr, h) }()

	// Ждём, пока сервер поднимется.
	var lastErr error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var resp *http.Response
		if resp, lastErr = http.Get("http://" + addr + "/"); lastErr == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("server did not start on %s: %v", addr, lastErr)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}

	// После Shutdown сервер больше не принимает соединения.
	if resp, err := http.Get("http://" + addr + "/"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server still accepts connections after shutdown")
	}
}
