package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"practice/logging"
)

// decodeLogs разбирает строки JSON-лога (одна запись — одна строка).
func decodeLogs(t *testing.T, buf *bytes.Buffer) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("невалидная строка JSON-лога %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func rawField(t *testing.T, m map[string]json.RawMessage, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("в записи нет поля %q: %v", key, m)
	}
	return string(v)
}

func newRedactLogger(t *testing.T, secrets []string) (*bytes.Buffer, *slog.Logger) {
	t.Helper()
	buf := &bytes.Buffer{}
	base := slog.NewJSONHandler(buf, nil)
	h := logging.NewRedactHandler(base, secrets)
	return buf, slog.New(h)
}

func TestNewLoggerFiltersByLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	l := logging.NewLogger(buf, slog.LevelWarn)

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	logs := decodeLogs(t, buf)
	if len(logs) != 2 {
		t.Fatalf("записей: %d, want 2 — Debug/Info должны быть отфильтрованы уровнем", len(logs))
	}
	if got := rawField(t, logs[0], "level"); got != `"WARN"` {
		t.Errorf("level[0] = %s, want %q", got, `"WARN"`)
	}
	if got := rawField(t, logs[1], "level"); got != `"ERROR"` {
		t.Errorf("level[1] = %s, want %q", got, `"ERROR"`)
	}
}

func TestWithRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	l := logging.NewLogger(buf, slog.LevelInfo)
	l = logging.WithRequestID(l, "req-123")

	l.Info("started")

	logs := decodeLogs(t, buf)
	if len(logs) != 1 {
		t.Fatalf("записей: %d, want 1", len(logs))
	}
	if got := rawField(t, logs[0], "request_id"); got != `"req-123"` {
		t.Errorf("request_id = %s, want %q", got, `"req-123"`)
	}
}

func TestRedactHandlerRedactsAttrValue(t *testing.T) {
	buf, l := newRedactLogger(t, []string{"s3cr3t", "token-42"})

	l.Info("user logged in",
		"user", "alice",
		"api_key", "abc-s3cr3t-xyz",
		"note", "no secrets here",
	)

	logs := decodeLogs(t, buf)
	if len(logs) != 1 {
		t.Fatalf("записей: %d, want 1", len(logs))
	}
	if got := rawField(t, logs[0], "api_key"); got != `"abc-[REDACTED]-xyz"` {
		t.Errorf("api_key = %s, want %q (секрет в значении атрибута не заменён)", got, `"abc-[REDACTED]-xyz"`)
	}
	if got := rawField(t, logs[0], "user"); got != `"alice"` {
		t.Errorf("user = %s, want %q — несекретные атрибуты менять нельзя", got, `"alice"`)
	}
	if got := rawField(t, logs[0], "note"); got != `"no secrets here"` {
		t.Errorf("note = %s, want %q", got, `"no secrets here"`)
	}
}

func TestRedactHandlerRedactsMessage(t *testing.T) {
	buf, l := newRedactLogger(t, []string{"s3cr3t"})

	l.Info("password s3cr3t is compromised")

	logs := decodeLogs(t, buf)
	if got := rawField(t, logs[0], "msg"); got != `"password [REDACTED] is compromised"` {
		t.Errorf("msg = %s, want %q (секрет в сообщении не заменён)", got, `"password [REDACTED] is compromised"`)
	}
}

func TestRedactHandlerNestedGroup(t *testing.T) {
	buf, l := newRedactLogger(t, []string{"s3cr3t", "token-42"})

	l.Info("auth attempt", slog.Group("auth",
		slog.String("secret", "s3cr3t"),
		slog.String("user", "u1"),
		slog.Group("inner", slog.String("token", "x-token-42-y")),
	))

	logs := decodeLogs(t, buf)
	raw := rawField(t, logs[0], "auth")
	var auth struct {
		Secret string `json:"secret"`
		User   string `json:"user"`
		Inner  struct {
			Token string `json:"token"`
		} `json:"inner"`
	}
	if err := json.Unmarshal([]byte(raw), &auth); err != nil {
		t.Fatalf("auth — невалидный JSON: %v", err)
	}
	if auth.Secret != logging.Redacted {
		t.Errorf("auth.secret = %q, want %q — вложенная группа не обработана", auth.Secret, logging.Redacted)
	}
	if auth.Inner.Token != "x-[REDACTED]-y" {
		t.Errorf("auth.inner.token = %q, want %q", auth.Inner.Token, "x-[REDACTED]-y")
	}
	if auth.User != "u1" {
		t.Errorf("auth.user = %q, want %q", auth.User, "u1")
	}
}

func TestRedactHandlerWithAttrsChain(t *testing.T) {
	buf, l := newRedactLogger(t, []string{"s3cr3t"})

	l = l.With("api_key", "key-s3cr3t-1").With("env", "prod")
	l.Info("hello")

	logs := decodeLogs(t, buf)
	if len(logs) != 1 {
		t.Fatalf("записей: %d, want 1", len(logs))
	}
	if got := rawField(t, logs[0], "api_key"); got != `"key-[REDACTED]-1"` {
		t.Errorf("api_key = %s, want %q — WithAttrs не маскирует секрет", got, `"key-[REDACTED]-1"`)
	}
	if got := rawField(t, logs[0], "env"); got != `"prod"` {
		t.Errorf("env = %s, want %q — WithAttrs сломал цепочку атрибутов", got, `"prod"`)
	}
}
