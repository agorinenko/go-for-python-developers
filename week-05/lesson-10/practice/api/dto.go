// Package api — DTO и (де)сериализация REST API сервиса заказов.
// В капстоуне это часть orders/internal/httpapi: контракт «Неделя 5»
// (POST /v1/orders, GET /v1/orders/{id}, GET /v1/orders) и единая схема
// ошибок {"error": {"code": "...", "message": "..."}}.
package api

import "time"

// Item — позиция заказа в запросе и ответах.
// Name опционален в запросе (имя проставит каталог, неделя 7), поэтому omitempty.
type Item struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name,omitempty"`
	Quantity  int    `json:"quantity"`
	Price     int64  `json:"price"` // в копейках, никогда не float64
}

// CreateOrderRequest — тело POST /v1/orders.
type CreateOrderRequest struct {
	CustomerID string `json:"customer_id"`
	Items      []Item `json:"items"`
}

// CreateOrderResponse — тело ответа 201 на создание заказа.
type CreateOrderResponse struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Items      []Item    `json:"items"`
	Status     string    `json:"status"`
	Total      int64     `json:"total"`
	CreatedAt  time.Time `json:"created_at"`
}

// OrderResponse — тело ответа 200 на получение заказа.
// Сейчас идентично CreateOrderResponse: разнесены заранее, чтобы контракты
// могли эволюционировать независимо (DTO против доменного типа — лекция).
type OrderResponse struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Items      []Item    `json:"items"`
	Status     string    `json:"status"`
	Total      int64     `json:"total"`
	CreatedAt  time.Time `json:"created_at"`
}

// ErrorBody — тело ошибки по контракту CAPSTONE.md.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse — единая схема ошибок API.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}
