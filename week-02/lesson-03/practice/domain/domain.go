// Package domain — минимальная копия доменной модели капстоуна.
// В капстоуне используется orders/internal/domain (неделя 1); здесь — копия,
// чтобы практика была автономным модулем. Код ГОТОВЫЙ: менять не нужно.
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// OrderStatus — жизненный цикл заказа.
type OrderStatus string

const (
	StatusNew       OrderStatus = "new"
	StatusValidated OrderStatus = "validated"
	StatusPaid      OrderStatus = "paid"
	StatusCancelled OrderStatus = "cancelled"
)

// OrderItem — позиция заказа. Деньги — int64 копеек, никогда не float64.
type OrderItem struct {
	ProductID string
	Name      string
	Quantity  int
	Price     int64 // в копейках
}

// Order — заказ.
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Status     OrderStatus
	Total      int64 // в копейках
	CreatedAt  time.Time
}

// Sentinel-ошибки домена: вызывающий код проверяет их через errors.Is.
var (
	ErrEmptyCustomer     = errors.New("orders: customer id is empty")
	ErrNoItems           = errors.New("orders: order has no items")
	ErrInvalidItem       = errors.New("orders: invalid item")
	ErrOrderNotFound     = errors.New("orders: order not found")
	ErrInvalidTransition = errors.New("orders: invalid status transition")
)

// ListFilter — фильтр списка заказов. Контракт CAPSTONE.md, неделя 2.
type ListFilter struct {
	CustomerID string
	Status     OrderStatus
	Limit      int
}

// OrderRepository — интерфейс хранилища заказов. Контракт CAPSTONE.md,
// неделя 2. Сервисный слой зависит от ЭТОГО интерфейса, а не от конкретного
// хранилища (PostgreSQL появится только на неделе 6).
type OrderRepository interface {
	Save(ctx context.Context, o *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, f ListFilter) ([]*Order, error)
	UpdateStatus(ctx context.Context, id string, s OrderStatus) error
}

// OrderValidator — валидация одной позиции заказа. Контракт CAPSTONE.md,
// неделя 2.
type OrderValidator interface {
	Validate(ctx context.Context, item OrderItem) error
}

// Amount — сумма позиции (для удобства вызывающего кода).
func (i OrderItem) Amount() int64 {
	return int64(i.Quantity) * i.Price
}

// ApplyStatus применяет новый статус заказа.
// Допустимые переходы: new → validated → paid; new → cancelled.
// Недопустимый переход — ErrInvalidTransition с указанием обоих статусов.
// Готовый код недели 1: сервисный слой пользуется им при отмене заказа.
func (o *Order) ApplyStatus(next OrderStatus) error {
	switch o.Status {
	case StatusNew:
		if next != StatusValidated && next != StatusCancelled {
			return fmt.Errorf("%w: %v → %v", ErrInvalidTransition, o.Status, next)
		}
	case StatusValidated:
		if next != StatusPaid {
			return fmt.Errorf("%w: %v → %v", ErrInvalidTransition, o.Status, next)
		}
	default:
		return fmt.Errorf("%w: %v → %v", ErrInvalidTransition, o.Status, next)
	}
	o.Status = next
	return nil
}
