package httpapi

import (
	"context"
	"net/http"
)

// Run запускает HTTP-сервер на addr и останавливает его по отмене ctx
// (graceful shutdown).
//
// TODO (лекция, раздел «Graceful shutdown»):
//  1. сконструируй http.Server с таймаутами — дефолты опасны (Slowloris):
//     ReadHeaderTimeout: 5 * time.Second,
//     ReadTimeout:       15 * time.Second,
//     WriteTimeout:      30 * time.Second,
//     IdleTimeout:       60 * time.Second;
//  2. запусти srv.ListenAndServe() в отдельной горутине, ошибку отправь
//     в буферизованный канал (cap 1);
//  3. жди select:
//     - ctx.Done(): вызови srv.Shutdown с контекстом с таймаутом 10s,
//     верни его ошибку;
//     - err из канала: если errors.Is(err, http.ErrServerClosed) — это
//     нормальное завершение (верни nil), иначе верни err;
//  4. не блокируйся навсегда: Shutdown обязан завершиться по своему таймауту.
//
// Сейчас заглушка возвращает nil, не запуская сервер, — тест TestRun падает.
func Run(ctx context.Context, addr string, h http.Handler) error {
	// TODO: реализуй.
	return nil
}
