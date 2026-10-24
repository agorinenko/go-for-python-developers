// Package repository — PostgreSQL-репозиторий заказов.
// В капстоуне это orders/internal/storage/postgres (контракт «Неделя 6»),
// реализация domain.OrderRepository поверх *sql.DB.
package repository

import (
	"context"
	"database/sql"

	"practice/domain"
)

// SQL-запросы. Схема зафиксирована в README практики:
// таблицы orders (id, customer_id, status, total, created_at) и
// order_items (order_id, product_id, name, quantity, price).
// Константы экспортированы, потому что на них опираются тесты; в капстоуне
// их можно сделать неэкспортируемыми.
const (
	InsertOrderSQL  = "INSERT INTO orders (id, customer_id, status, total, created_at) VALUES ($1, $2, $3, $4, $5)"
	InsertItemSQL   = "INSERT INTO order_items (order_id, product_id, name, quantity, price) VALUES ($1, $2, $3, $4, $5)"
	SelectOrderSQL  = "SELECT id, customer_id, status, total, created_at FROM orders WHERE id = $1"
	SelectItemsSQL  = "SELECT product_id, name, quantity, price FROM order_items WHERE order_id = $1"
	UpdateStatusSQL = "UPDATE orders SET status = $2 WHERE id = $1"
	BaseListSQL     = "SELECT id, customer_id, status, total, created_at FROM orders"
)

// PostgresOrderRepository — реализация репозитория заказов поверх *sql.DB.
type PostgresOrderRepository struct {
	db *sql.DB
}

// NewPostgresOrderRepository создаёт репозиторий.
func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

// Save сохраняет заказ вместе с позициями в одной транзакции:
// либо сохранены оба (заказ + все позиции), либо ничего.
//
// TODO (лекция, раздел «Транзакции»):
//  1. tx, err := r.db.BeginTx(ctx, nil);
//  2. defer tx.Rollback() — после успешного Commit() он вернёт
//     sql.ErrTxDone, это нормально, ошибку игнорируй;
//  3. tx.ExecContext(ctx, InsertOrderSQL, o.ID, o.CustomerID,
//     string(o.Status), o.Total, o.CreatedAt) — именованные типы
//     (OrderStatus) конвертируй в string: драйвер принимает только
//     базовые типы;
//  4. для каждой позиции — tx.ExecContext(ctx, InsertItemSQL, o.ID,
//     it.ProductID, it.Name, it.Quantity, it.Price);
//  5. ошибки оборачивай контекстом (fmt.Errorf("save order %s: %w", ...));
//  6. в конце — tx.Commit().
//
// Сейчас заглушка возвращает nil: sqlmock не видит ни одного вызова,
// и ExpectationsWereMet роняет тесты.
func (r *PostgresOrderRepository) Save(ctx context.Context, o *domain.Order) error {
	// TODO: реализуй.
	return nil
}

// GetByID возвращает заказ с позициями; неизвестный id —
// domain.ErrOrderNotFound (совместимо с errors.Is).
//
// TODO (лекция, разделы «Запросы» и «sql.ErrNoRows»):
//  1. QueryRowContext(ctx, SelectOrderSQL, id) → Scan;
//  2. если errors.Is(err, sql.ErrNoRows) — верни domain.ErrOrderNotFound;
//  3. QueryContext(ctx, SelectItemsSQL, id) — собери позиции, закрой rows
//     (defer rows.Close()) и проверь rows.Err() после цикла;
//  4. ошибки оборачивай: fmt.Errorf("get order %s: %w", id, err).
//
// Сейчас заглушка всегда возвращает ErrOrderNotFound — тест «найден»
// падает именно поэтому.
func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	// TODO: реализуй.
	return nil, domain.ErrOrderNotFound
}

// List возвращает заказы по фильтру, отсортированные по created_at DESC.
//
// Контракт SQL (на него опираются тесты, порядок фиксирован):
//  1. запрос начинается с BaseListSQL;
//  2. если CustomerID задан — добавляется " WHERE customer_id = $N";
//  3. если Status задан — добавляется " WHERE status = $N" или
//     " AND status = $N", если WHERE уже есть;
//  4. в конце всегда: " ORDER BY created_at DESC LIMIT $N";
//  5. Limit <= 0 → лимит по умолчанию 50;
//  6. аргументы — в том же порядке: customerID?, status?, limit.
//
// TODO (лекция, раздел «Динамические запросы»):
//  1. собери query и args по контракту выше (нумерация плейсхолдеров — по
//     len(args)+1, или считай вручную);
//  2. QueryContext → Scan строк → закрыть rows → rows.Err();
//  3. пустой результат — пустой не-nil слайс, не nil.
//
// Сейчас заглушка возвращает nil — тесты падают на содержимом результата.
func (r *PostgresOrderRepository) List(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error) {
	// TODO: реализуй.
	return nil, nil
}

// UpdateStatus меняет статус заказа; неизвестный id — ErrOrderNotFound.
//
// TODO (лекция, раздел «Обновления»):
//  1. ExecContext(ctx, UpdateStatusSQL, id, string(status));
//  2. если RowsAffected() == 0 — верни domain.ErrOrderNotFound;
//  3. ошибки оборачивай контекстом.
//
// Сейчас заглушка возвращает nil — тест «неизвестный id» падает.
func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	// TODO: реализуй.
	return nil
}
