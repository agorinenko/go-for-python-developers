// Package repository — PostgreSQL-репозиторий заказов.
// ГОТОВАЯ реализация: это код урока 11 (там он был заданием, здесь —
// инструмент для интеграционных тестов). В капстоуне пакет переносится
// в orders/internal/storage/postgres и переводится на pgxpool.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"practice/domain"
)

// SQL-запросы репозитория.
const (
	InsertOrderSQL  = "INSERT INTO orders (id, customer_id, status, total, created_at) VALUES ($1, $2, $3, $4, $5)"
	InsertItemSQL   = "INSERT INTO order_items (order_id, product_id, name, quantity, price) VALUES ($1, $2, $3, $4, $5)"
	SelectOrderSQL  = "SELECT id, customer_id, status, total, created_at FROM orders WHERE id = $1"
	SelectItemsSQL  = "SELECT product_id, name, quantity, price FROM order_items WHERE order_id = $1"
	UpdateStatusSQL = "UPDATE orders SET status = $2 WHERE id = $1"
	BaseListSQL     = "SELECT id, customer_id, status, total, created_at FROM orders"
)

const defaultListLimit = 50

// PostgresOrderRepository — реализация репозитория заказов поверх *sql.DB.
type PostgresOrderRepository struct {
	db *sql.DB
}

// NewPostgresOrderRepository создаёт репозиторий.
func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

// Save сохраняет заказ вместе с позициями в одной транзакции.
func (r *PostgresOrderRepository) Save(ctx context.Context, o *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save order %s: begin tx: %w", o.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, InsertOrderSQL, o.ID, o.CustomerID, string(o.Status), o.Total, o.CreatedAt); err != nil {
		return fmt.Errorf("save order %s: insert order: %w", o.ID, err)
	}
	for _, it := range o.Items {
		if _, err := tx.ExecContext(ctx, InsertItemSQL, o.ID, it.ProductID, it.Name, it.Quantity, it.Price); err != nil {
			return fmt.Errorf("save order %s: insert item %s: %w", o.ID, it.ProductID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save order %s: commit: %w", o.ID, err)
	}
	return nil
}

// GetByID возвращает заказ с позициями; неизвестный id — domain.ErrOrderNotFound.
func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	o := &domain.Order{}
	if err := r.db.QueryRowContext(ctx, SelectOrderSQL, id).
		Scan(&o.ID, &o.CustomerID, &o.Status, &o.Total, &o.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("get order %s: %w", id, err)
	}

	rows, err := r.db.QueryContext(ctx, SelectItemsSQL, id)
	if err != nil {
		return nil, fmt.Errorf("get order %s: query items: %w", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Quantity, &it.Price); err != nil {
			return nil, fmt.Errorf("get order %s: scan item: %w", id, err)
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get order %s: iterate items: %w", id, err)
	}
	return o, nil
}

// List возвращает заказы по фильтру, отсортированные по created_at DESC.
func (r *PostgresOrderRepository) List(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	query := BaseListSQL
	args := make([]any, 0, 3)
	if f.CustomerID != "" {
		query += " WHERE customer_id = $" + strconv.Itoa(len(args)+1)
		args = append(args, f.CustomerID)
	}
	if f.Status != "" {
		if len(args) > 0 {
			query += " AND status = $" + strconv.Itoa(len(args)+1)
		} else {
			query += " WHERE status = $" + strconv.Itoa(len(args)+1)
		}
		args = append(args, string(f.Status))
	}
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]*domain.Order, 0)
	for rows.Next() {
		o := &domain.Order{}
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.Total, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("list orders: scan: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list orders: iterate: %w", err)
	}
	return orders, nil
}

// UpdateStatus меняет статус заказа; неизвестный id — ErrOrderNotFound.
func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	res, err := r.db.ExecContext(ctx, UpdateStatusSQL, id, string(status))
	if err != nil {
		return fmt.Errorf("update order %s status: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update order %s status: %w", id, err)
	}
	if n == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}
