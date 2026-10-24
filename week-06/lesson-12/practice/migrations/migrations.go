// Package migrations содержит goose-миграции схемы сервиса заказов.
// В капстоуне это orders/internal/migrations (контракт «Неделя 6»).
package migrations

import "embed"

// FS — встроенные файлы миграций. Использование:
//
//	goose.SetBaseFS(migrations.FS)
//	goose.SetDialect("postgres")
//	goose.Up(db, ".")
//
//go:embed *.sql
var FS embed.FS
