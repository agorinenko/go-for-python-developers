package grpcapi

// Этот файл — рукописный эквивалент того, что генерирует buf/protoc из
// orders.proto: интерфейс сервиса, регистрация в grpc.Server и
// glue-хэндлеры (распаковка/упаковка сообщений). В реальном проекте
// такой код НИКОГДА не пишут руками — он генерируется. Здесь он нужен,
// чтобы практика работала без protoc. Менять не нужно.

import (
	"context"

	"google.golang.org/grpc"
)

// Полные имена методов (используются в ServiceDesc и клиентом).
const (
	OrdersService_CreateOrder_FullMethodName = "/orders.v1.OrdersService/CreateOrder"
	OrdersService_GetOrder_FullMethodName    = "/orders.v1.OrdersService/GetOrder"
	OrdersService_ListOrders_FullMethodName  = "/orders.v1.OrdersService/ListOrders"
)

// OrdersServiceServer — интерфейс, который реализует наш сервер.
type OrdersServiceServer interface {
	CreateOrder(ctx context.Context, in *CreateOrderRequest) (*CreateOrderResponse, error)
	GetOrder(ctx context.Context, in *GetOrderRequest) (*GetOrderResponse, error)
	ListOrders(in *ListOrdersRequest, stream OrdersService_ListOrdersServer) error
}

// OrdersService_ListOrdersServer — поток ответов ListOrders (server-streaming):
// сервер вызывает Send для каждого сообщения; grpc.ServerStream даёт
// Context() — контекст стрима, отменяемый клиентом.
type OrdersService_ListOrdersServer interface {
	Send(*OrderResponse) error
	grpc.ServerStream
}

// RegisterOrdersServiceServer регистрирует реализацию в grpc.Server.
func RegisterOrdersServiceServer(s *grpc.Server, impl OrdersServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "orders.v1.OrdersService",
		HandlerType: (*OrdersServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "CreateOrder", Handler: ordersServiceCreateOrderHandler},
			{MethodName: "GetOrder", Handler: ordersServiceGetOrderHandler},
		},
		Streams: []grpc.StreamDesc{
			{StreamName: "ListOrders", Handler: ordersServiceListOrdersHandler, ServerStreams: true},
		},
	}, impl)
}

func ordersServiceCreateOrderHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CreateOrderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrdersServiceServer).CreateOrder(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: OrdersService_CreateOrder_FullMethodName}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(OrdersServiceServer).CreateOrder(ctx, req.(*CreateOrderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func ordersServiceGetOrderHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetOrderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrdersServiceServer).GetOrder(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: OrdersService_GetOrder_FullMethodName}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(OrdersServiceServer).GetOrder(ctx, req.(*GetOrderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func ordersServiceListOrdersHandler(srv any, stream grpc.ServerStream) error {
	m := new(ListOrdersRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(OrdersServiceServer).ListOrders(m, &ordersServiceListOrdersServer{ServerStream: stream})
}

// ordersServiceListOrdersServer — адаптер ServerStream под интерфейс
// OrdersService_ListOrdersServer.
type ordersServiceListOrdersServer struct {
	grpc.ServerStream
}

func (x *ordersServiceListOrdersServer) Send(resp *OrderResponse) error {
	return x.ServerStream.SendMsg(resp)
}
