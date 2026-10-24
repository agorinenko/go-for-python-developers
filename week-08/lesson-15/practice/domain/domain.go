// Package domain — минимальная копия доменной модели капстоуна.
// Каноническая версия живёт в orders/internal/domain (CAPSTONE.md, неделя 1);
// в практике оставлено только то, что нужно событиям.
package domain

import "time"

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

// NewOrder создаёт заказ в статусе StatusNew и пересчитывает Total.
// Готовый код — событиям нужен только уже собранный заказ.
func NewOrder(id, customerID string, items []OrderItem) *Order {
	o := &Order{
		ID:         id,
		CustomerID: customerID,
		Items:      items,
		Status:     StatusNew,
		CreatedAt:  time.Now(),
	}
	o.RecalcTotal()
	return o
}

// RecalcTotal пересчитывает Total заказа по позициям.
func (o *Order) RecalcTotal() {
	var total int64
	for _, it := range o.Items {
		total += int64(it.Quantity) * it.Price
	}
	o.Total = total
}
