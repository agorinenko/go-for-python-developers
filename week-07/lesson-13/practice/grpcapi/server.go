package grpcapi

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"

	"practice/domain"
	"practice/service"
)

// jsonCodec — JSON-кодек gRPC для рукописных сообщений этой практики.
// Сгенерированные protobuf-сообщения работают со стандартным proto-кодеком,
// поэтому в капстоуне кодек не нужен; здесь он — плата за автономность
// (рукописные структуры не реализуют proto.Message).
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string { return "json" }

// Codec — кодек, который тесты применяют к вызовам через grpc.ForceCodec.
// Это классический (устаревающий) интерфейс encoding.Codec — его всё ещё
// поддерживает grpc-go; новый CodecV2 требует работу с mem.BufferSlice,
// что для учебной практики избыточно.
var Codec encoding.Codec = jsonCodec{}

func init() {
	encoding.RegisterCodec(Codec)
}

// Service — интерфейс сервисного слоя заказов (реализует *service.OrderService).
type Service interface {
	CreateOrder(ctx context.Context, customerID string, items []domain.OrderItem) (*domain.Order, error)
	GetOrder(ctx context.Context, id string) (*domain.Order, error)
	ListOrders(ctx context.Context, f service.ListFilter) ([]*domain.Order, error)
}

// Server — реализация OrdersServiceServer поверх сервисного слоя.
type Server struct {
	svc Service
}

// NewServer создаёт gRPC-сервер заказов.
func NewServer(svc Service) *Server {
	return &Server{svc: svc}
}

// CreateOrder — unary-метод создания заказа.
//
// TODO (лекция, разделы «Сервер» и «Маппинг ошибок»):
//  1. провалидируй запрос и верни status.Error(codes.InvalidArgument, ...)
//     с внятным текстом:
//     - customer_id пуст → "customer_id is required";
//     - items пусты → "items must not be empty";
//     - у позиции quantity <= 0 или price <= 0 → "items[i]: quantity must
//     be > 0" / "items[i]: price must be > 0";
//  2. смапь Items (int32 → int) в []domain.OrderItem и вызови
//     svc.CreateOrder(ctx, req.CustomerID, items);
//  3. замапь ошибки сервиса (лекция, раздел «Маппинг ошибок»):
//     domain.ErrEmptyCustomer / domain.ErrNoItems / domain.ErrInvalidItem
//     → codes.InvalidArgument,
//     domain.ErrInvalidTransition → codes.AlreadyExists (конфликт
//     состояния: заказ уже не в том статусе),
//     domain.ErrOrderNotFound → codes.NotFound,
//     context.Canceled / context.DeadlineExceeded →
//     status.FromContextError(err).Err(),
//     всё остальное → codes.Internal;
//  4. успех: &CreateOrderResponse{Order: toOrderResponse(o)}.
//
// Сейчас заглушка отвечает Unimplemented — тесты падают на коде ошибки.
func (s *Server) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	// TODO: реализуй.
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// GetOrder — unary-метод получения заказа по id.
//
// TODO (лекция, раздел «Маппинг ошибок»):
//  1. вызови svc.GetOrder(ctx, req.Id);
//  2. domain.ErrOrderNotFound → status.Error(codes.NotFound,
//     "order not found"); остальные ошибки → codes.Internal;
//  3. успех: &GetOrderResponse{Order: toOrderResponse(o)}.
//
// Сейчас заглушка отвечает Unimplemented — тесты падают на коде ошибки.
func (s *Server) GetOrder(ctx context.Context, req *GetOrderRequest) (*GetOrderResponse, error) {
	// TODO: реализуй.
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// ListOrders — server-streaming: все заказы по фильтру, порциями.
//
// TODO (лекция, раздел «Server-streaming»):
//  1. ctx := stream.Context() — контекст стрима (отменяется клиентом);
//  2. смапь фильтр: service.ListFilter{CustomerID: ..., Status: ..., Limit: int(...)};
//  3. вызови svc.ListOrders(ctx, f); ошибка — если errors.Is(err,
//     context.Canceled) || errors.Is(err, context.DeadlineExceeded),
//     верни status.FromContextError(err).Err(), иначе codes.Internal;
//  4. отправляй заказы чанками по 2 (stream.Send) — учебный акцент на
//     потоковости; между Send проверяй ctx.Err() и при отмене верни
//     status.FromContextError(ctx.Err()).Err();
//  5. после отправки всех заказов — верни nil (gRPC сам закроет поток).
//
// Сейчас заглушка отвечает Unimplemented — тесты падают на коде ошибки.
func (s *Server) ListOrders(req *ListOrdersRequest, stream OrdersService_ListOrdersServer) error {
	// TODO: реализуй.
	return status.Error(codes.Unimplemented, "not implemented")
}

// toOrderResponse мапит доменный заказ в gRPC-сообщение. Готовая функция.
func toOrderResponse(o *domain.Order) *OrderResponse {
	resp := &OrderResponse{
		Id:         o.ID,
		CustomerID: o.CustomerID,
		Items:      make([]*OrderResponse_Item, len(o.Items)),
		Status:     string(o.Status),
		Total:      o.Total,
		CreatedAt:  o.CreatedAt.Format(time.RFC3339),
	}
	for i, it := range o.Items {
		resp.Items[i] = &OrderResponse_Item{
			ProductID: it.ProductID,
			Name:      it.Name,
			Quantity:  int32(it.Quantity),
			Price:     it.Price,
		}
	}
	return resp
}
