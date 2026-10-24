package orders_test

import (
	"errors"
	"testing"

	"practice/orders"
)

func validItems() []orders.OrderItem {
	return []orders.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
		{ProductID: "p-2", Name: "Ручка", Quantity: 5, Price: 50},
	}
}

func TestNewOrder(t *testing.T) {
	o, err := orders.NewOrder("o-1", "c-1", validItems())
	if err != nil {
		t.Fatalf("NewOrder: unexpected error: %v", err)
	}
	if o.ID != "o-1" {
		t.Errorf("ID = %q, want %q", o.ID, "o-1")
	}
	if o.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", o.CustomerID, "c-1")
	}
	if o.Status != orders.StatusNew {
		t.Errorf("Status = %q, want %q", o.Status, orders.StatusNew)
	}
	if want := int64(2*1_000 + 5*50); o.Total != want {
		t.Errorf("Total = %d, want %d", o.Total, want)
	}
	if o.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero value")
	}
}

func TestNewOrderErrors(t *testing.T) {
	cases := []struct {
		name       string
		customerID string
		items      []orders.OrderItem
		wantErr    error
	}{
		{"empty customer", "", validItems(), orders.ErrEmptyCustomer},
		{"no items", "c-1", nil, orders.ErrNoItems},
		{"zero quantity", "c-1", []orders.OrderItem{{ProductID: "p-1", Quantity: 0, Price: 10}}, orders.ErrInvalidItem},
		{"negative price", "c-1", []orders.OrderItem{{ProductID: "p-1", Quantity: 1, Price: -5}}, orders.ErrInvalidItem},
		{"empty product id", "c-1", []orders.OrderItem{{Quantity: 1, Price: 10}}, orders.ErrInvalidItem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := orders.NewOrder("o-1", tc.customerID, tc.items)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewOrder error = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

func TestRecalcTotal(t *testing.T) {
	o := &orders.Order{Items: validItems()}
	o.RecalcTotal()
	if want := int64(2*1_000 + 5*50); o.Total != want {
		t.Fatalf("RecalcTotal: Total = %d, want %d", o.Total, want)
	}
}

func TestApplyStatusValidTransitions(t *testing.T) {
	valid := []struct {
		from, to orders.OrderStatus
	}{
		{orders.StatusNew, orders.StatusValidated},
		{orders.StatusValidated, orders.StatusPaid},
		{orders.StatusNew, orders.StatusCancelled},
	}
	for _, tc := range valid {
		o := &orders.Order{Status: tc.from}
		if err := o.ApplyStatus(tc.to); err != nil {
			t.Errorf("ApplyStatus(%v → %v): unexpected error: %v", tc.from, tc.to, err)
		}
		if o.Status != tc.to {
			t.Errorf("ApplyStatus(%v → %v): Status = %v", tc.from, tc.to, o.Status)
		}
	}
}

func TestApplyStatusInvalidTransitions(t *testing.T) {
	invalid := []struct {
		from, to orders.OrderStatus
	}{
		{orders.StatusNew, orders.StatusPaid},       // пропуск validated
		{orders.StatusPaid, orders.StatusValidated}, // назад нельзя
		{orders.StatusValidated, orders.StatusNew},  // назад нельзя
		{orders.StatusCancelled, orders.StatusPaid}, // из cancelled никуда
		{orders.StatusPaid, orders.StatusCancelled}, // оплаченный не отменяется
	}
	for _, tc := range invalid {
		o := &orders.Order{Status: tc.from}
		if err := o.ApplyStatus(tc.to); !errors.Is(err, orders.ErrInvalidTransition) {
			t.Errorf("ApplyStatus(%v → %v) error = %v, want ErrInvalidTransition", tc.from, tc.to, err)
		}
		if o.Status != tc.from {
			t.Errorf("ApplyStatus(%v → %v): статус изменился на %v, должен остаться %v", tc.from, tc.to, o.Status, tc.from)
		}
	}
}

func TestValidate(t *testing.T) {
	valid := &orders.Order{ID: "o-1", CustomerID: "c-1", Items: validItems()}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate(valid): unexpected error: %v", err)
	}

	noCustomer := &orders.Order{ID: "o-1", Items: validItems()}
	if err := noCustomer.Validate(); !errors.Is(err, orders.ErrEmptyCustomer) {
		t.Errorf("Validate(no customer) = %v, want ErrEmptyCustomer", err)
	}

	noItems := &orders.Order{ID: "o-1", CustomerID: "c-1"}
	if err := noItems.Validate(); !errors.Is(err, orders.ErrNoItems) {
		t.Errorf("Validate(no items) = %v, want ErrNoItems", err)
	}
}
