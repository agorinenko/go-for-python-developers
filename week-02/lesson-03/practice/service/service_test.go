package service_test

import (
	"context"
	"errors"
	"testing"

	"practice/domain"
	"practice/service"
)

// fakeRepo — фейковый репозиторий в памяти: рабочая реализация интерфейса
// domain.OrderRepository, но без базы данных. Живёт только в тестах.
type fakeRepo struct {
	orders  map[string]*domain.Order
	updates []statusUpdate // лог вызовов UpdateStatus
}

type statusUpdate struct {
	id     string
	status domain.OrderStatus
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{orders: make(map[string]*domain.Order)}
}

func (f *fakeRepo) Save(_ context.Context, o *domain.Order) error {
	f.orders[o.ID] = o
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*domain.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (f *fakeRepo) List(_ context.Context, flt domain.ListFilter) ([]*domain.Order, error) {
	var out []*domain.Order
	for _, o := range f.orders {
		if flt.CustomerID != "" && o.CustomerID != flt.CustomerID {
			continue
		}
		if flt.Status != "" && o.Status != flt.Status {
			continue
		}
		out = append(out, o)
		if flt.Limit > 0 && len(out) >= flt.Limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, id string, s domain.OrderStatus) error {
	o, ok := f.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	o.Status = s
	f.updates = append(f.updates, statusUpdate{id: id, status: s})
	return nil
}

// fakeValidator — фейковый валидатор: отклоняет позиции, перечисленные в reject.
type fakeValidator struct {
	reject map[string]error // ProductID -> ошибка
}

// errRejected — пример ошибки «внешней» системы (например, сервиса каталога).
var errRejected = errors.New("catalog: product is out of stock")

func (v *fakeValidator) Validate(_ context.Context, item domain.OrderItem) error {
	return v.reject[item.ProductID]
}

func validItems() []domain.OrderItem {
	return []domain.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
		{ProductID: "p-2", Name: "Ручка", Quantity: 5, Price: 50},
	}
}

func newService(repo domain.OrderRepository, v domain.OrderValidator) *service.OrderService {
	if v == nil {
		v = &fakeValidator{}
	}
	return service.NewOrderService(repo, v)
}

// idsOf превращает слайс заказов в множество ID — для сравнения без учёта
// порядка (fake-репозиторий хранит заказы в map, порядок не гарантирован).
func idsOf(orders []*domain.Order) map[string]bool {
	ids := make(map[string]bool, len(orders))
	for _, o := range orders {
		ids[o.ID] = true
	}
	return ids
}

func TestCreateOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, nil)

	o, err := svc.CreateOrder(context.Background(), "c-1", validItems())
	if err != nil {
		t.Fatalf("CreateOrder: unexpected error: %v", err)
	}
	if o.ID == "" {
		t.Error("CreateOrder: пустой ID")
	}
	if o.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", o.CustomerID, "c-1")
	}
	if o.Status != domain.StatusNew {
		t.Errorf("Status = %q, want %q", o.Status, domain.StatusNew)
	}
	if want := int64(2*1_000 + 5*50); o.Total != want {
		t.Errorf("Total = %d, want %d", o.Total, want)
	}
	if o.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero value")
	}

	// заказ действительно сохранён в репозитории
	got, err := repo.GetByID(context.Background(), o.ID)
	if err != nil {
		t.Fatalf("GetByID после CreateOrder: unexpected error: %v", err)
	}
	if got == nil || got.ID != o.ID {
		t.Errorf("репозиторий не содержит созданный заказ: got %v", got)
	}
}

func TestCreateOrderErrors(t *testing.T) {
	cases := []struct {
		name       string
		customerID string
		items      []domain.OrderItem
		wantErr    error
	}{
		{"пустой клиент", "", validItems(), domain.ErrEmptyCustomer},
		{"нет позиций", "c-1", nil, domain.ErrNoItems},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := newService(repo, nil)
			_, err := svc.CreateOrder(context.Background(), tc.customerID, tc.items)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateOrder error = %v, want errors.Is %v", err, tc.wantErr)
			}
			if len(repo.orders) != 0 {
				t.Fatalf("заказ сохранён, хотя создание не удалось: %d записей", len(repo.orders))
			}
		})
	}
}

func TestCreateOrderValidatorError(t *testing.T) {
	repo := newFakeRepo()
	v := &fakeValidator{reject: map[string]error{"p-2": errRejected}}
	svc := newService(repo, v)

	_, err := svc.CreateOrder(context.Background(), "c-1", validItems())
	if err == nil {
		t.Fatal("CreateOrder: ошибка валидатора проигнорирована")
	}
	// ошибка должна сохранять связь и с доменной ошибкой, и с причиной
	if !errors.Is(err, domain.ErrInvalidItem) {
		t.Errorf("ошибка потеряла доменный контекст: %v (want errors.Is %v)", err, domain.ErrInvalidItem)
	}
	if !errors.Is(err, errRejected) {
		t.Errorf("ошибка валидатора не проброшена: %v (want errors.Is %v)", err, errRejected)
	}
	if len(repo.orders) != 0 {
		t.Fatalf("заказ сохранён, хотя валидация не прошла: %d записей", len(repo.orders))
	}
}

func TestGetOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, nil)
	created, err := svc.CreateOrder(context.Background(), "c-1", validItems())
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	t.Run("существующий", func(t *testing.T) {
		got, err := svc.GetOrder(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("GetOrder: unexpected error: %v", err)
		}
		if got == nil || got.ID != created.ID {
			t.Fatalf("GetOrder = %v, want заказ %s", got, created.ID)
		}
	})

	t.Run("неизвестный ID", func(t *testing.T) {
		_, err := svc.GetOrder(context.Background(), "missing")
		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Fatalf("GetOrder error = %v, want errors.Is %v", err, domain.ErrOrderNotFound)
		}
	})
}

func TestCancelOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, nil)
	o, err := svc.CreateOrder(context.Background(), "c-1", validItems())
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	if err := svc.CancelOrder(context.Background(), o.ID); err != nil {
		t.Fatalf("CancelOrder: unexpected error: %v", err)
	}

	got, err := svc.GetOrder(context.Background(), o.ID)
	if err != nil {
		t.Fatalf("GetOrder после CancelOrder: %v", err)
	}
	if got.Status != domain.StatusCancelled {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusCancelled)
	}
	// статус должен сохраняться через UpdateStatus, а не мутацией напрямую
	if len(repo.updates) != 1 {
		t.Fatalf("UpdateStatus вызван %d раз, want 1", len(repo.updates))
	}
	if repo.updates[0].id != o.ID || repo.updates[0].status != domain.StatusCancelled {
		t.Errorf("UpdateStatus(%q, %q), want (%q, %q)",
			repo.updates[0].id, repo.updates[0].status, o.ID, domain.StatusCancelled)
	}
}

func TestCancelOrderErrors(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, nil)

	t.Run("неизвестный ID", func(t *testing.T) {
		if err := svc.CancelOrder(context.Background(), "missing"); !errors.Is(err, domain.ErrOrderNotFound) {
			t.Fatalf("CancelOrder error = %v, want errors.Is %v", err, domain.ErrOrderNotFound)
		}
	})

	t.Run("повторная отмена", func(t *testing.T) {
		o, err := svc.CreateOrder(context.Background(), "c-1", validItems())
		if err != nil {
			t.Fatalf("CreateOrder: %v", err)
		}
		if err := svc.CancelOrder(context.Background(), o.ID); err != nil {
			t.Fatalf("первая CancelOrder: %v", err)
		}
		if err := svc.CancelOrder(context.Background(), o.ID); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("вторая CancelOrder error = %v, want errors.Is %v", err, domain.ErrInvalidTransition)
		}
	})
}

func TestListOrders(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, nil)
	ctx := context.Background()

	o1, err := svc.CreateOrder(ctx, "c-1", validItems())
	if err != nil {
		t.Fatalf("CreateOrder o1: %v", err)
	}
	o2, err := svc.CreateOrder(ctx, "c-1", validItems())
	if err != nil {
		t.Fatalf("CreateOrder o2: %v", err)
	}
	o3, err := svc.CreateOrder(ctx, "c-2", validItems())
	if err != nil {
		t.Fatalf("CreateOrder o3: %v", err)
	}
	if err := svc.CancelOrder(ctx, o2.ID); err != nil {
		t.Fatalf("CancelOrder o2: %v", err)
	}

	cases := []struct {
		name   string
		filter domain.ListFilter
		want   map[string]bool
	}{
		{"пустой фильтр", domain.ListFilter{}, idsOf([]*domain.Order{o1, o2, o3})},
		{"по клиенту", domain.ListFilter{CustomerID: "c-1"}, idsOf([]*domain.Order{o1, o2})},
		{"по статусу", domain.ListFilter{Status: domain.StatusCancelled}, idsOf([]*domain.Order{o2})},
		{"клиент + статус", domain.ListFilter{CustomerID: "c-1", Status: domain.StatusCancelled}, idsOf([]*domain.Order{o2})},
		{"нет совпадений", domain.ListFilter{CustomerID: "c-99"}, map[string]bool{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ListOrders(ctx, tc.filter)
			if err != nil {
				t.Fatalf("ListOrders: unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ListOrders вернул %d заказов, want %d", len(got), len(tc.want))
			}
			for _, o := range got {
				if !tc.want[o.ID] {
					t.Errorf("заказ %s не должен попадать в результат фильтра", o.ID)
				}
			}
		})
	}

	t.Run("лимит", func(t *testing.T) {
		got, err := svc.ListOrders(ctx, domain.ListFilter{Limit: 1})
		if err != nil {
			t.Fatalf("ListOrders: unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("ListOrders(Limit=1) вернул %d заказов, want 1", len(got))
		}
		// какой именно заказ вернёт map-хранилище — не определено;
		// важно, что он существует в хранилище
		if !idsOf([]*domain.Order{o1, o2, o3})[got[0].ID] {
			t.Errorf("заказ %s отсутствует в хранилище", got[0].ID)
		}
	})
}
