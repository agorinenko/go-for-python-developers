// Package domain — учебная копия доменной модели капстоуна
// (orders/internal/domain, неделя 1). Копия нужна, чтобы практика была
// автономным модулем. Код готовый: в капстоуне он уже написан, менять
// здесь ничего не нужно.
package domain

import (
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
	Price     int64
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

// Sentinel-ошибки домена: проверяются через errors.Is.
var (
	ErrEmptyCustomer     = errors.New("orders: customer id is empty")
	ErrNoItems           = errors.New("orders: order has no items")
	ErrInvalidItem       = errors.New("orders: invalid item")
	ErrOrderNotFound     = errors.New("orders: order not found")
	ErrInvalidTransition = errors.New("orders: invalid status transition")
)

// NewOrder создаёт заказ в статусе StatusNew.
func NewOrder(id, customerID string, items []OrderItem) (*Order, error) {
	o := &Order{
		ID:         id,
		CustomerID: customerID,
		Items:      items,
		Status:     StatusNew,
		CreatedAt:  time.Now(),
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	o.RecalcTotal()
	return o, nil
}

// Validate проверяет инварианты заказа.
func (o *Order) Validate() error {
	if o.CustomerID == "" {
		return ErrEmptyCustomer
	}
	if len(o.Items) == 0 {
		return ErrNoItems
	}
	for _, it := range o.Items {
		if it.ProductID == "" {
			return fmt.Errorf("%w: product id is empty", ErrInvalidItem)
		}
		if it.Quantity <= 0 {
			return fmt.Errorf("%w: product %q: quantity must be > 0", ErrInvalidItem, it.ProductID)
		}
		if it.Price < 0 {
			return fmt.Errorf("%w: product %q: price must be >= 0", ErrInvalidItem, it.ProductID)
		}
	}
	return nil
}

// RecalcTotal пересчитывает Total по позициям.
func (o *Order) RecalcTotal() {
	var total int64
	for _, it := range o.Items {
		total += it.Amount()
	}
	o.Total = total
}

// ApplyStatus применяет переход статуса.
// Допустимо: new → validated → paid; new → cancelled.
func (o *Order) ApplyStatus(next OrderStatus) error {
	allowed := map[OrderStatus][]OrderStatus{
		StatusNew:       {StatusValidated, StatusCancelled},
		StatusValidated: {StatusPaid},
	}
	for _, ok := range allowed[o.Status] {
		if next == ok {
			o.Status = next
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, o.Status, next)
}

// Amount — сумма позиции.
func (i OrderItem) Amount() int64 {
	return int64(i.Quantity) * i.Price
}
