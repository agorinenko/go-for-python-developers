package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"practice/domain"
	"practice/repository"
)

var createdAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func testOrder() *domain.Order {
	return &domain.Order{
		ID:         "o-1",
		CustomerID: "c-1",
		Items: []domain.OrderItem{
			{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
			{ProductID: "p-2", Name: "Ручка", Quantity: 5, Price: 50},
		},
		Status:    domain.StatusNew,
		Total:     2_250,
		CreatedAt: createdAt,
	}
}

func newMockRepo(t *testing.T) (*repository.PostgresOrderRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return repository.NewPostgresOrderRepository(db), mock
}

func TestSave(t *testing.T) {
	repo, mock := newMockRepo(t)
	o := testOrder()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(repository.InsertOrderSQL)).
		WithArgs("o-1", "c-1", "new", int64(2_250), createdAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, it := range o.Items {
		mock.ExpectExec(regexp.QuoteMeta(repository.InsertItemSQL)).
			WithArgs("o-1", it.ProductID, it.Name, it.Quantity, it.Price).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	if err := repo.Save(context.Background(), o); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestSaveRollsBackOnError(t *testing.T) {
	repo, mock := newMockRepo(t)
	o := testOrder()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(repository.InsertOrderSQL)).
		WithArgs("o-1", "c-1", "new", int64(2_250), createdAt).
		WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	if err := repo.Save(context.Background(), o); err == nil {
		t.Fatal("Save: expected error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("rollback must be called: %v", err)
	}
}

func TestGetByID(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(regexp.QuoteMeta(repository.SelectOrderSQL)).
		WithArgs("o-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_id", "status", "total", "created_at"}).
			AddRow("o-1", "c-1", "new", int64(2_250), createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(repository.SelectItemsSQL)).
		WithArgs("o-1").
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "name", "quantity", "price"}).
			AddRow("p-1", "Книга", 2, int64(1_000)).
			AddRow("p-2", "Ручка", 5, int64(50)))

	o, err := repo.GetByID(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}
	if o.ID != "o-1" || o.CustomerID != "c-1" || o.Status != domain.StatusNew || o.Total != 2_250 {
		t.Errorf("order = %+v", o)
	}
	if !o.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", o.CreatedAt, createdAt)
	}
	if len(o.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(o.Items))
	}
	if o.Items[0].ProductID != "p-1" || o.Items[0].Quantity != 2 || o.Items[0].Price != 1_000 {
		t.Errorf("Items[0] = %+v", o.Items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(regexp.QuoteMeta(repository.SelectOrderSQL)).
		WithArgs("nope").
		WillReturnError(sql.ErrNoRows)

	if _, err := repo.GetByID(context.Background(), "nope"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("GetByID error = %v, want errors.Is(ErrOrderNotFound)", err)
	}
}

func TestList(t *testing.T) {
	cases := []struct {
		name    string
		filter  domain.ListFilter
		wantSQL string
		wantArg []driver.Value
	}{
		{
			name:    "limit only",
			filter:  domain.ListFilter{Limit: 10},
			wantSQL: repository.BaseListSQL + " ORDER BY created_at DESC LIMIT $1",
			wantArg: []driver.Value{10},
		},
		{
			name:    "by customer",
			filter:  domain.ListFilter{CustomerID: "c-1", Limit: 10},
			wantSQL: repository.BaseListSQL + " WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2",
			wantArg: []driver.Value{"c-1", 10},
		},
		{
			name:    "by customer and status",
			filter:  domain.ListFilter{CustomerID: "c-1", Status: domain.StatusNew, Limit: 10},
			wantSQL: repository.BaseListSQL + " WHERE customer_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3",
			wantArg: []driver.Value{"c-1", "new", 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newMockRepo(t)

			mock.ExpectQuery(regexp.QuoteMeta(tc.wantSQL)).
				WithArgs(tc.wantArg...).
				WillReturnRows(sqlmock.NewRows([]string{"id", "customer_id", "status", "total", "created_at"}).
					AddRow("o-1", "c-1", "new", int64(2_250), createdAt).
					AddRow("o-2", "c-1", "new", int64(100), createdAt))

			orders, err := repo.List(context.Background(), tc.filter)
			if err != nil {
				t.Fatalf("List: unexpected error: %v", err)
			}
			if len(orders) != 2 {
				t.Fatalf("len(orders) = %d, want 2", len(orders))
			}
			if orders[0].ID != "o-1" {
				t.Errorf("orders[0].ID = %q, want %q", orders[0].ID, "o-1")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestListEmptyResultIsNotEmptyNil(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(regexp.QuoteMeta(repository.BaseListSQL + " ORDER BY created_at DESC LIMIT $1")).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_id", "status", "total", "created_at"}))

	orders, err := repo.List(context.Background(), domain.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if orders == nil {
		t.Fatal("List returned nil for empty result, want empty non-nil slice")
	}
	if len(orders) != 0 {
		t.Fatalf("len(orders) = %d, want 0", len(orders))
	}
}

func TestUpdateStatus(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(regexp.QuoteMeta(repository.UpdateStatusSQL)).
		WithArgs("o-1", "paid").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), "o-1", domain.StatusPaid); err != nil {
		t.Fatalf("UpdateStatus: unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(regexp.QuoteMeta(repository.UpdateStatusSQL)).
		WithArgs("nope", "paid").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.UpdateStatus(context.Background(), "nope", domain.StatusPaid); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("UpdateStatus error = %v, want errors.Is(ErrOrderNotFound)", err)
	}
}
