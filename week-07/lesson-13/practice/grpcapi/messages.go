// Package grpcapi — gRPC API сервиса заказов.
// В капстоуне это orders/internal/grpcapi поверх кода, сгенерированного
// из api/orders/v1/orders.proto (buf/protoc). В этой практике генерация
// заменена РУКОПИСНЫМ эквивалентом: структуры и интерфейсы ниже
// поле-в-поле повторяют то, что произвёл бы protoc (см. orders.proto
// в корне практики). Так практика остаётся автономной — protoc не нужен.
package grpcapi

// CreateOrderRequest — запрос создания заказа.
type CreateOrderRequest struct {
	CustomerID string
	Items      []*CreateOrderRequest_Item
}

// CreateOrderRequest_Item — позиция в запросе (в сгенерированном коде —
// вложенное сообщение CreateOrderRequest.Item).
type CreateOrderRequest_Item struct {
	ProductID string
	Quantity  int32
	Price     int64 // в копейках
}

// CreateOrderResponse — ответ на создание заказа.
type CreateOrderResponse struct {
	Order *OrderResponse
}

// GetOrderRequest — запрос заказа по id (поле называется Id — как в
// сгенерированном protoc'ом коде).
type GetOrderRequest struct {
	Id string
}

// GetOrderResponse — ответ с заказом.
type GetOrderResponse struct {
	Order *OrderResponse
}

// ListOrdersRequest — запрос списка заказов (server-streaming).
type ListOrdersRequest struct {
	CustomerID string
	Status     string
	Limit      int32
}

// OrderResponse — заказ в ответах gRPC.
type OrderResponse struct {
	Id         string
	CustomerID string
	Items      []*OrderResponse_Item
	Status     string
	Total      int64
	// CreatedAt — RFC 3339 строка. В настоящем сгенерированном коде это
	// *timestamppb.Timestamp; строкой заменено ради автономности практики
	// (без зависимости от protobuf-типов).
	CreatedAt string
}

// OrderResponse_Item — позиция заказа в ответах.
type OrderResponse_Item struct {
	ProductID string
	Name      string
	Quantity  int32
	Price     int64 // в копейках
}
