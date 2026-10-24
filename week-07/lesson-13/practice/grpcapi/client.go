package grpcapi

// Рукописный эквивалент сгенерированного клиента OrdersServiceClient —
// нужен только для тестов этой практики. В реальном проекте его
// генерирует buf из orders.proto.

import (
	"context"

	"google.golang.org/grpc"
)

// OrdersServiceClient — клиентская сторона сервиса.
type OrdersServiceClient interface {
	CreateOrder(ctx context.Context, in *CreateOrderRequest, opts ...grpc.CallOption) (*CreateOrderResponse, error)
	GetOrder(ctx context.Context, in *GetOrderRequest, opts ...grpc.CallOption) (*GetOrderResponse, error)
	ListOrders(ctx context.Context, in *ListOrdersRequest, opts ...grpc.CallOption) (OrdersService_ListOrdersClient, error)
}

// OrdersService_ListOrdersClient — поток ответов ListOrders: Recv до io.EOF.
type OrdersService_ListOrdersClient interface {
	Recv() (*OrderResponse, error)
	CloseSend() error
}

type ordersServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewOrdersServiceClient создаёт клиент поверх соединения.
func NewOrdersServiceClient(cc grpc.ClientConnInterface) OrdersServiceClient {
	return &ordersServiceClient{cc: cc}
}

func (c *ordersServiceClient) CreateOrder(ctx context.Context, in *CreateOrderRequest, opts ...grpc.CallOption) (*CreateOrderResponse, error) {
	out := new(CreateOrderResponse)
	if err := c.cc.Invoke(ctx, OrdersService_CreateOrder_FullMethodName, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ordersServiceClient) GetOrder(ctx context.Context, in *GetOrderRequest, opts ...grpc.CallOption) (*GetOrderResponse, error) {
	out := new(GetOrderResponse)
	if err := c.cc.Invoke(ctx, OrdersService_GetOrder_FullMethodName, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ordersServiceClient) ListOrders(ctx context.Context, in *ListOrdersRequest, opts ...grpc.CallOption) (OrdersService_ListOrdersClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{StreamName: "ListOrders", ServerStreams: true}, OrdersService_ListOrders_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &ordersServiceListOrdersClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ordersServiceListOrdersClient struct {
	grpc.ClientStream
}

func (x *ordersServiceListOrdersClient) Recv() (*OrderResponse, error) {
	m := new(OrderResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
