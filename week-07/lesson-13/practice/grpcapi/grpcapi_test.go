package grpcapi_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"practice/domain"
	"practice/grpcapi"
	"practice/service"
)

const bufSize = 1024 * 1024

// startServer поднимает gRPC-сервер поверх in-memory транспорта bufconn
// (без реальных сокетов) и возвращает клиента.
func startServer(t *testing.T, svc grpcapi.Service) grpcapi.OrdersServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	grpcapi.RegisterOrdersServiceServer(srv, grpcapi.NewServer(svc))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// рукописные сообщения не proto.Message — используем JSON-кодек
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcapi.Codec)),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return grpcapi.NewOrdersServiceClient(conn)
}

func validItems() []*grpcapi.CreateOrderRequest_Item {
	return []*grpcapi.CreateOrderRequest_Item{
		{ProductID: "p-1", Quantity: 2, Price: 1_000},
		{ProductID: "p-2", Quantity: 5, Price: 50},
	}
}

func TestCreateOrder(t *testing.T) {
	client := startServer(t, service.NewOrderService())

	resp, err := client.CreateOrder(context.Background(), &grpcapi.CreateOrderRequest{
		CustomerID: "c-1",
		Items:      validItems(),
	})
	if err != nil {
		t.Fatalf("CreateOrder: unexpected error: %v", err)
	}
	if resp.Order == nil {
		t.Fatal("CreateOrder: resp.Order is nil")
	}
	if resp.Order.Id == "" {
		t.Error("CreateOrder: Id is empty")
	}
	if resp.Order.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", resp.Order.CustomerID, "c-1")
	}
	if resp.Order.Status != string(domain.StatusNew) {
		t.Errorf("Status = %q, want %q", resp.Order.Status, domain.StatusNew)
	}
	if resp.Order.Total != 2_250 {
		t.Errorf("Total = %d, want %d", resp.Order.Total, 2_250)
	}
	if _, err := time.Parse(time.RFC3339, resp.Order.CreatedAt); err != nil {
		t.Errorf("CreatedAt %q не парсится как RFC 3339: %v", resp.Order.CreatedAt, err)
	}
	if len(resp.Order.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(resp.Order.Items))
	}
}

func TestCreateOrderInvalid(t *testing.T) {
	client := startServer(t, service.NewOrderService())

	cases := []struct {
		name string
		req  *grpcapi.CreateOrderRequest
	}{
		{"empty customer", &grpcapi.CreateOrderRequest{CustomerID: "", Items: validItems()}},
		{"no items", &grpcapi.CreateOrderRequest{CustomerID: "c-1"}},
		{"zero quantity", &grpcapi.CreateOrderRequest{CustomerID: "c-1", Items: []*grpcapi.CreateOrderRequest_Item{{ProductID: "p-1", Quantity: 0, Price: 10}}}},
		{"negative price", &grpcapi.CreateOrderRequest{CustomerID: "c-1", Items: []*grpcapi.CreateOrderRequest_Item{{ProductID: "p-1", Quantity: 1, Price: -5}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CreateOrder(context.Background(), tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("CreateOrder error = %v, want codes.InvalidArgument", err)
			}
		})
	}
}

// fakeService — фейк сервисного слоя для сценариев с ошибками.
type fakeService struct {
	createErr error
	listFn    func(ctx context.Context, f service.ListFilter) ([]*domain.Order, error)
	listCalls chan struct{} // закрывается при каждом вызове ListOrders
}

func (f *fakeService) CreateOrder(context.Context, string, []domain.OrderItem) (*domain.Order, error) {
	return nil, f.createErr
}

func (f *fakeService) GetOrder(context.Context, string) (*domain.Order, error) {
	return nil, domain.ErrOrderNotFound
}

func (f *fakeService) ListOrders(ctx context.Context, f2 service.ListFilter) ([]*domain.Order, error) {
	if f.listCalls != nil {
		close(f.listCalls)
	}
	if f.listFn != nil {
		return f.listFn(ctx, f2)
	}
	return nil, nil
}

func TestCreateOrderConflict(t *testing.T) {
	client := startServer(t, &fakeService{createErr: domain.ErrInvalidTransition})

	_, err := client.CreateOrder(context.Background(), &grpcapi.CreateOrderRequest{
		CustomerID: "c-1",
		Items:      validItems(),
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("CreateOrder error = %v, want codes.AlreadyExists", err)
	}
}

func TestGetOrder(t *testing.T) {
	svc := service.NewOrderService()
	client := startServer(t, svc)

	created, err := svc.CreateOrder(context.Background(), "c-1", []domain.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp, err := client.GetOrder(context.Background(), &grpcapi.GetOrderRequest{Id: created.ID})
	if err != nil {
		t.Fatalf("GetOrder: unexpected error: %v", err)
	}
	if resp.Order == nil || resp.Order.Id != created.ID {
		t.Fatalf("GetOrder: got %+v, want id %q", resp.Order, created.ID)
	}
	if resp.Order.Items[0].Name != "Книга" {
		t.Errorf("Items[0].Name = %q, want %q", resp.Order.Items[0].Name, "Книга")
	}
}

func TestGetOrderNotFound(t *testing.T) {
	client := startServer(t, service.NewOrderService())

	_, err := client.GetOrder(context.Background(), &grpcapi.GetOrderRequest{Id: "nope"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetOrder error = %v, want codes.NotFound", err)
	}
}

func TestListOrdersStreaming(t *testing.T) {
	svc := service.NewOrderService()
	client := startServer(t, svc)

	for i := 1; i <= 5; i++ {
		if _, err := svc.CreateOrder(context.Background(), "c-1", []domain.OrderItem{
			{ProductID: "p-1", Quantity: 1, Price: 100},
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	stream, err := client.ListOrders(context.Background(), &grpcapi.ListOrdersRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}

	var got []string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		got = append(got, resp.Id)
	}
	if len(got) != 5 {
		t.Fatalf("received %d orders, want 5 (ids: %v)", len(got), got)
	}
	// сервис сортирует по id — порядок детерминирован
	if got[0] != "o-1" || got[4] != "o-5" {
		t.Errorf("order of ids = %v, want o-1..o-5", got)
	}
}

// TestListOrdersCancellation проверяет корректный выход из стрима при
// отмене контекста клиента: сервер обязан вызвать сервисный слой и
// вернуть codes.Canceled (а не уронить стрим или ответить Unknown).
func TestListOrdersCancellation(t *testing.T) {
	blocking := &fakeService{
		listCalls: make(chan struct{}),
		listFn: func(ctx context.Context, _ service.ListFilter) ([]*domain.Order, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	client := startServer(t, blocking)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.ListOrders(ctx, &grpcapi.ListOrdersRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}

	// Сначала дождёмся, пока сервер дойдёт до сервисного слоя
	// (иначе заглушка отвечает Unimplemented, а этот select не дождётся
	// закрытия канала), и только потом отменяем контекст.
	select {
	case <-blocking.listCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("svc.ListOrders не был вызван сервером")
	}

	cancel()

	if _, err := stream.Recv(); status.Code(err) != codes.Canceled {
		t.Fatalf("Recv after cancel = %v, want codes.Canceled", err)
	}
}
