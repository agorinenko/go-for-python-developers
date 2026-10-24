package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"practice/api"
	"practice/domain"
)

const validBody = `{"customer_id":"c-1","items":[{"product_id":"p-1","name":"Книга","quantity":2,"price":1000},{"product_id":"p-2","quantity":5,"price":50}]}`

func TestDecodeCreateOrderValid(t *testing.T) {
	req, err := api.DecodeCreateOrder(strings.NewReader(validBody))
	if err != nil {
		t.Fatalf("DecodeCreateOrder: unexpected error: %v", err)
	}
	if req.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", req.CustomerID, "c-1")
	}
	if len(req.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(req.Items))
	}
	if req.Items[0].ProductID != "p-1" || req.Items[0].Quantity != 2 || req.Items[0].Price != 1000 {
		t.Errorf("Items[0] = %+v", req.Items[0])
	}
	if req.Items[1].Name != "" {
		t.Errorf("Items[1].Name = %q, want empty (поле отсутствовало)", req.Items[1].Name)
	}
}

func TestDecodeCreateOrderValidation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr error
		wantMsg string
	}{
		{
			name:    "empty customer",
			body:    `{"customer_id":"","items":[{"product_id":"p-1","quantity":1,"price":10}]}`,
			wantErr: domain.ErrEmptyCustomer,
			wantMsg: "customer_id",
		},
		{
			name:    "no items key",
			body:    `{"customer_id":"c-1"}`,
			wantErr: domain.ErrNoItems,
			wantMsg: "items",
		},
		{
			name:    "items null",
			body:    `{"customer_id":"c-1","items":null}`,
			wantErr: domain.ErrNoItems,
			wantMsg: "items",
		},
		{
			name:    "zero quantity",
			body:    `{"customer_id":"c-1","items":[{"product_id":"p-1","quantity":0,"price":10}]}`,
			wantErr: domain.ErrInvalidItem,
			wantMsg: "quantity",
		},
		{
			name:    "negative price",
			body:    `{"customer_id":"c-1","items":[{"product_id":"p-1","quantity":1,"price":-5}]}`,
			wantErr: domain.ErrInvalidItem,
			wantMsg: "price",
		},
		{
			name:    "zero price",
			body:    `{"customer_id":"c-1","items":[{"product_id":"p-1","quantity":1,"price":0}]}`,
			wantErr: domain.ErrInvalidItem,
			wantMsg: "price",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := api.DecodeCreateOrder(strings.NewReader(tc.body))
			if err == nil {
				t.Fatalf("DecodeCreateOrder(%s): expected error, got nil", tc.body)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantMsg)
			}
		})
	}
}

func TestDecodeCreateOrderUnknownField(t *testing.T) {
	body := `{"customer_id":"c-1","items":[{"product_id":"p-1","quantity":1,"price":10}],"extra_field":1}`
	if _, err := api.DecodeCreateOrder(strings.NewReader(body)); err == nil {
		t.Fatal("unknown field must be rejected (DisallowUnknownFields)")
	}
}

func TestDecodeCreateOrderTrailingJSON(t *testing.T) {
	body := validBody + `{"customer_id":"c-2"}`
	if _, err := api.DecodeCreateOrder(strings.NewReader(body)); err == nil {
		t.Fatal("second JSON value in the stream must be rejected")
	}
}

func TestDecodeCreateOrderMalformedJSON(t *testing.T) {
	if _, err := api.DecodeCreateOrder(strings.NewReader(`{not json`)); err == nil {
		t.Fatal("malformed JSON must be rejected")
	}
}

func TestToDomain(t *testing.T) {
	req, err := api.DecodeCreateOrder(strings.NewReader(validBody))
	if err != nil {
		t.Fatalf("DecodeCreateOrder: %v", err)
	}
	o, err := req.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain: unexpected error: %v", err)
	}
	if o.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", o.CustomerID, "c-1")
	}
	if len(o.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(o.Items))
	}
	if o.Status != domain.StatusNew {
		t.Errorf("Status = %q, want %q", o.Status, domain.StatusNew)
	}
	if want := int64(2*1000 + 5*50); o.Total != want {
		t.Errorf("Total = %d, want %d", o.Total, want)
	}
	if o.ID != "" {
		t.Errorf("ID = %q, want empty (его проставит сервис)", o.ID)
	}
}

func TestToDomainInvalid(t *testing.T) {
	req := api.CreateOrderRequest{
		CustomerID: "c-1",
		Items:      []api.Item{{ProductID: "p-1", Quantity: 1, Price: 10}, {Quantity: 2, Price: 10}},
	}
	if _, err := req.ToDomain(); !errors.Is(err, domain.ErrInvalidItem) {
		t.Fatalf("ToDomain error = %v, want errors.Is(ErrInvalidItem)", err)
	}

	if _, err := (api.CreateOrderRequest{Items: []api.Item{{ProductID: "p-1", Quantity: 1, Price: 10}}}).ToDomain(); !errors.Is(err, domain.ErrEmptyCustomer) {
		t.Fatalf("ToDomain(no customer) error = %v, want ErrEmptyCustomer", err)
	}
}

func TestEncodeOrder(t *testing.T) {
	o := &domain.Order{
		ID:         "o-1",
		CustomerID: "c-1",
		Items: []domain.OrderItem{
			{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
			{ProductID: "p-2", Quantity: 5, Price: 50},
		},
		Status:    domain.StatusNew,
		Total:     2_250,
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	var buf bytes.Buffer
	if err := api.EncodeOrder(&buf, o); err != nil {
		t.Fatalf("EncodeOrder: %v", err)
	}

	var got api.OrderResponse
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("encoded output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got.ID != "o-1" {
		t.Errorf("ID = %q, want %q", got.ID, "o-1")
	}
	if got.CustomerID != "c-1" {
		t.Errorf("CustomerID = %q, want %q", got.CustomerID, "c-1")
	}
	if len(got.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(got.Items))
	}
	if got.Items[0].ProductID != "p-1" || got.Items[0].Name != "Книга" || got.Items[0].Quantity != 2 || got.Items[0].Price != 1_000 {
		t.Errorf("Items[0] = %+v", got.Items[0])
	}
	if got.Status != string(domain.StatusNew) {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusNew)
	}
	if got.Total != 2_250 {
		t.Errorf("Total = %d, want %d", got.Total, 2_250)
	}
	if !got.CreatedAt.Equal(o.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, o.CreatedAt)
	}
}

func TestWriteErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string // пустое = «не проверяем текст, но он должен быть непуст»
	}{
		{"not found", domain.ErrOrderNotFound, 404, "not_found", ""},
		{"invalid item", domain.ErrInvalidItem, 400, "bad_request", ""},
		{"empty customer", domain.ErrEmptyCustomer, 400, "bad_request", ""},
		{"no items", domain.ErrNoItems, 400, "bad_request", ""},
		{"invalid transition", domain.ErrInvalidTransition, 409, "conflict", ""},
		{"unknown error", errors.New("connection refused to postgres at 10.0.0.5"), 500, "internal", "internal error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			api.WriteError(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var er api.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
				t.Fatalf("body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
			}
			if er.Error.Code != tc.wantCode {
				t.Errorf("error.code = %q, want %q", er.Error.Code, tc.wantCode)
			}
			if tc.wantMsg != "" {
				if er.Error.Message != tc.wantMsg {
					t.Errorf("error.message = %q, want %q", er.Error.Message, tc.wantMsg)
				}
			} else if er.Error.Message == "" {
				t.Errorf("error.message is empty for %v", tc.err)
			}
		})
	}
}

func TestWriteErrorDoesNotLeakDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	api.WriteError(rec, errors.New("dial tcp 10.0.0.5:5432: password 'hunter2' rejected"))
	if strings.Contains(rec.Body.String(), "10.0.0.5") || strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("500 response leaks internal details: %s", rec.Body.String())
	}
}
