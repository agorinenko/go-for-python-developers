// Package orders содержит доменную модель заказа.
// Контракт заморожен в CAPSTONE.md (неделя 1): сигнатуры менять нельзя.
package orders

import (
	"errors"
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

// Sentinel-ошибки домена: вызывающий код проверяет их через errors.Is.
var (
	ErrEmptyCustomer     = errors.New("orders: customer id is empty")
	ErrNoItems           = errors.New("orders: order has no items")
	ErrInvalidItem       = errors.New("orders: invalid item")
	ErrOrderNotFound     = errors.New("orders: order not found")
	ErrInvalidTransition = errors.New("orders: invalid status transition")
)

// NewOrder создаёт заказ в статусе StatusNew.
//
// TODO: реализуй:
//  1. пустой customerID -> ErrEmptyCustomer;
//  2. пустой items -> ErrNoItems;
//  3. каждая позиция валидна (Validate), иначе ErrInvalidItem с деталями
//     через fmt.Errorf("%w: ...");
//  4. Total = сумма Quantity*Price; CreatedAt = time.Now();
//  5. заказ возвращается по указателю, ошибка — как обычное значение.
func NewOrder(id, customerID string, items []OrderItem) (*Order, error) {
	// TODO: реализуй. Сейчас возвращается «сырой» заказ без валидации —
	// тесты на ошибки падают именно поэтому.
	return &Order{ID: id, CustomerID: customerID, Items: items, Status: StatusNew}, nil
}

// Validate проверяет инварианты заказа: непустой CustomerID, есть позиции,
// каждая позиция валидна (ProductID непустой, Quantity > 0, Price >= 0).
//
// TODO: реализуй, ошибки — ErrEmptyCustomer / ErrNoItems / ErrInvalidItem.
func (o *Order) Validate() error {
	// TODO: реализуй.
	return nil
}

// RecalcTotal пересчитывает Total заказа по позициям.
//
// TODO: реализуй: сумма Quantity*Price.
func (o *Order) RecalcTotal() {
	// TODO: реализуй.
}

// ApplyStatus применяет новый статус заказа.
// Допустимые переходы: new → validated → paid; new → cancelled.
// Недопустимый переход — ErrInvalidTransition с указанием обоих статусов.
//
// TODO: реализуй машину переходов и обновление поля Status.
func (o *Order) ApplyStatus(next OrderStatus) error {
	// TODO: реализуй.
	return nil
}

// Amount — сумма позиции (для удобства вызывающего кода).
func (i OrderItem) Amount() int64 {
	return int64(i.Quantity) * i.Price
}
