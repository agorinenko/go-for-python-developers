// Интеграционные тесты против реальной PostgreSQL в Docker (testcontainers).
// Если Docker недоступен — тесты пропускаются (t.Skip с пояснением).
package practice_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"practice/domain"
	"practice/migrations"
	"practice/repository"
)

type IntegrationSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	db        *sql.DB
	repo      *repository.PostgresOrderRepository
}

func TestIntegration(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

// SetupSuite поднимает PostgreSQL 16 в Docker, применяет goose-миграции
// и конструирует репозиторий. При недоступном Docker — Skip с пояснением.
func (s *IntegrationSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("orders"),
		postgres.WithUsername("orders"),
		postgres.WithPassword("orders"),
		testcontainers.WithLogger(tclog.TestLogger(s.T())),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		s.T().Skipf("Docker недоступен — пропускаю интеграционные тесты: %v", err)
	}
	s.container = container

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	db, err := sql.Open("pgx", dsn)
	s.Require().NoError(err)
	s.db = db

	goose.SetBaseFS(migrations.FS)
	goose.SetDialect("postgres")
	s.Require().NoError(goose.Up(db, "."), "применение миграций")

	s.repo = repository.NewPostgresOrderRepository(db)
}

func (s *IntegrationSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *IntegrationSuite) newOrder(id, customer string) *domain.Order {
	o, err := domain.NewOrder(id, customer, []domain.OrderItem{
		{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 1_000},
		{ProductID: "p-2", Name: "Ручка", Quantity: 5, Price: 50},
	})
	s.Require().NoError(err)
	return o
}

// TestMigrationsCreateTables проверяет, что миграции применились и все
// таблицы существуют (outbox — миграция 0002, ваша TODO).
func (s *IntegrationSuite) TestMigrationsCreateTables() {
	for _, table := range []string{"orders", "order_items", "outbox"} {
		var name string
		err := s.db.QueryRow(`
			SELECT table_name FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1`, table).Scan(&name)
		s.Require().NoErrorf(err, "таблица %q должна существовать — проверь миграцию", table)
	}
}

func (s *IntegrationSuite) TestSaveAndGetByIDRoundtrip() {
	ctx := context.Background()
	o := s.newOrder("rt-1", "c-1")
	s.Require().NoError(s.repo.Save(ctx, o))

	got, err := s.repo.GetByID(ctx, "rt-1")
	s.Require().NoError(err)
	s.Equal("rt-1", got.ID)
	s.Equal("c-1", got.CustomerID)
	s.Equal(domain.StatusNew, got.Status)
	s.Equal(int64(2_250), got.Total)
	s.Require().Len(got.Items, 2)
	s.Equal("p-1", got.Items[0].ProductID)
	s.Equal("Книга", got.Items[0].Name)
	s.Equal(2, got.Items[0].Quantity)
	s.Equal(int64(1_000), got.Items[0].Price)
	// timestamptz в PostgreSQL имеет микросекундную точность
	s.Equal(o.CreatedAt.Truncate(time.Microsecond).UTC(),
		got.CreatedAt.Truncate(time.Microsecond).UTC())
}

func (s *IntegrationSuite) TestGetByIDNotFound() {
	_, err := s.repo.GetByID(context.Background(), "missing")
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrOrderNotFound))
}

func (s *IntegrationSuite) TestUpdateStatus() {
	ctx := context.Background()
	s.Require().NoError(s.repo.Save(ctx, s.newOrder("upd-1", "c-1")))

	s.Require().NoError(s.repo.UpdateStatus(ctx, "upd-1", domain.StatusPaid))

	got, err := s.repo.GetByID(ctx, "upd-1")
	s.Require().NoError(err)
	s.Equal(domain.StatusPaid, got.Status)

	err = s.repo.UpdateStatus(ctx, "missing", domain.StatusPaid)
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrOrderNotFound))
}

func (s *IntegrationSuite) TestListFilters() {
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		o := s.newOrder(fmt.Sprintf("list-%d", i), "c-1")
		s.Require().NoError(s.repo.Save(ctx, o))
	}
	s.Require().NoError(s.repo.Save(ctx, s.newOrder("list-other", "c-2")))

	all, err := s.repo.List(ctx, domain.ListFilter{Limit: 50})
	s.Require().NoError(err)
	s.Len(all, 4)

	byCustomer, err := s.repo.List(ctx, domain.ListFilter{CustomerID: "c-1", Limit: 50})
	s.Require().NoError(err)
	s.Require().Len(byCustomer, 3)
	for _, o := range byCustomer {
		s.Equal("c-1", o.CustomerID)
	}

	limited, err := s.repo.List(ctx, domain.ListFilter{Limit: 2})
	s.Require().NoError(err)
	s.Len(limited, 2)

	byStatus, err := s.repo.List(ctx, domain.ListFilter{Status: domain.StatusNew, Limit: 50})
	s.Require().NoError(err)
	s.Len(byStatus, 4)
}
