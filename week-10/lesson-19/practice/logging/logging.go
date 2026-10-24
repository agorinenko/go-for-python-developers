// Package logging — структурное логирование сервиса заказов на slog с
// маскированием секретов (CAPSTONE.md, неделя 10: internal/telemetry/log).
package logging

import (
	"context"
	"io"
	"log/slog"
)

// Redacted — значение, которым заменяются секреты в логах.
const Redacted = "[REDACTED]"

// NewLogger создаёт JSON-логгер, пишущий в w, с минимальным уровнем level.
// Готовый код: это просто slog.New(slog.NewJSONHandler(...)).
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// WithRequestID возвращает логгер с атрибутом request_id: request-scoped
// логирование — все записи цепочки вызовов одного запроса получают этот
// атрибут и собираются в логах по нему (лекция, раздел «Request-scoped
// логгер»).
//
// TODO: реализуй через l.With("request_id", requestID).
func WithRequestID(l *slog.Logger, requestID string) *slog.Logger {
	// TODO: реализуй. Заглушка возвращает логгер без атрибута.
	return l
}

// RedactHandler — обёртка над slog.Handler, заменяющая секреты на Redacted
// в значениях атрибутов (включая вложенные группы) и в тексте сообщения.
type RedactHandler struct {
	next    slog.Handler
	secrets []string
}

// NewRedactHandler создаёт маскирующий handler поверх next.
func NewRedactHandler(next slog.Handler, secrets []string) *RedactHandler {
	return &RedactHandler{next: next, secrets: secrets}
}

// Enabled делегирует проверку уровня следующему handler'у.
func (h *RedactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle записывает запись, предварительно заменив секреты.
//
// TODO: реализуй маскирование (лекция, раздел «Маскирование секретов»):
//  1. создай запись без атрибутов: r2 := slog.NewRecord(r.Time, r.Level,
//     r.Message, r.PC) — НЕ r.Clone(): Clone копирует старые атрибуты,
//     и AddAttrs добавит к ним дубликаты (в сыром JSON останутся секреты!);
//  2. замени r2.Message — каждый секрет из h.secrets на Redacted через
//     strings.ReplaceAll;
//  3. собери новые атрибуты: пройди по r.Attrs(); строковое значение —
//     заменить, значение-группу (slog.KindGroup) — обработать РЕКУРСИВНО;
//     удобно вынести приватный метод
//     func (h *RedactHandler) redactValue(v slog.Value) slog.Value
//     (группа: slog.GroupValue(обработанные attr...));
//  4. r2.AddAttrs(новые атрибуты...); верни h.next.Handle(ctx, r2).
func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
	// TODO: реализуй маскирование. Заглушка пишет запись как есть.
	return h.next.Handle(ctx, r)
}

// WithAttrs возвращает handler с предварительно прикреплёнными атрибутами,
// в которых секреты тоже заменены.
//
// TODO: реализуй: замени секреты в attrs (тем же способом, что в Handle) и
// верни &RedactHandler{next: h.next.WithAttrs(обработанные), secrets: h.secrets}.
// Атрибуты и их порядок менять нельзя — только значения.
func (h *RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// TODO: реализуй маскирование. Заглушка делегирует без замены.
	return &RedactHandler{next: h.next.WithAttrs(attrs), secrets: h.secrets}
}

// WithGroup открывает группу в следующем handler'е, сохраняя маскирование:
// атрибуты, добавленные внутрь группы, пройдут через Handle как группа —
// рекурсивная обработка в Handle их накроет.
func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{next: h.next.WithGroup(name), secrets: h.secrets}
}
