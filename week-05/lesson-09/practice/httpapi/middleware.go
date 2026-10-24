package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

// RequestID гарантирует, что у каждого запроса есть идентификатор:
// если клиент не передал X-Request-ID — генерирует новый (32 hex-символа
// из crypto/rand) и проставляет его в заголовок ответа. Это внешний слой:
// заголовок должен появиться в ответе, даже если хэндлер ниже запаниковал.
// Реализация готова — используй её как образец формы middleware.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			var buf [16]byte
			if _, err := rand.Read(buf[:]); err != nil {
				id = "unknown"
			} else {
				id = hex.EncodeToString(buf[:])
			}
			r.Header.Set("X-Request-ID", id)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// Recover перехватывает panic из нижележащих хэндлеров: логирует панику
// и отвечает 500 вместо того, чтобы net/http молча разорвал соединение.
//
// TODO (лекция, раздел «Recovery middleware»):
//  1. defer + recover(): если recover() вернул nil — паники не было, просто выйди;
//  2. залогируй панику через slog.ErrorContext(r.Context(), "panic recovered",
//     "panic", rec);
//  3. ответь http.Error(w, "internal error", http.StatusInternalServerError).
//
// Сейчас заглушка просто передаёт запрос дальше — паника добирается до теста.
func Recover(next http.Handler) http.Handler {
	// TODO: реализуй.
	return next
}

// Logging возвращает middleware, который пишет структурированный лог
// о каждом запросе: method, path, status, duration и request_id.
// Это фабрика (как chi's middleware.RequestLogger): зависимость (логгер)
// фиксируется один раз, результат подставляется в цепочку Wrap.
// Статус известен только после выполнения хэндлера, поэтому нужна обёртка
// ResponseWriter, запоминающая первый WriteHeader.
//
// TODO (лекция, раздел «Логирование»):
//  1. верни func(next http.Handler) http.Handler — внутри:
//  2. объяви тип statusWriter struct { http.ResponseWriter; status int },
//     переопредели WriteHeader(status int), чтобы запомнить его (только первый)
//     и вызвать вложенный; переопредели Write, проставляя status = 200,
//     если WriteHeader ещё не вызывался;
//  3. start := time.Now(); оберни w; вызови next.ServeHTTP(sw, r);
//  4. если sw.status == 0 — считай 200;
//  5. log.InfoContext(r.Context(), "http request",
//     "method", r.Method, "path", r.URL.Path, "status", sw.status,
//     "duration", time.Since(start), "request_id", r.Header.Get("X-Request-ID")).
//
// Сейчас заглушка возвращает middleware, который просто передаёт запрос дальше.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	// TODO: реализуй.
	return func(next http.Handler) http.Handler {
		return next
	}
}

// Wrap возвращает h, обёрнутый middleware в порядке перечисления:
// первый элемент списка — самый внешний слой (выполняется первым).
// NewRouter использует Wrap для сборки цепочки.
//
// TODO (лекция, раздел «Цепочки middleware»):
// реализуй обёртывание: иди от последнего элемента списка к первому,
// на каждом шаге h = mw(h). Сейчас заглушка игнорирует middleware
// и возвращает h как есть.
func Wrap(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	// TODO: реализуй.
	return h
}
