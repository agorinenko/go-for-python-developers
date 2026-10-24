// Package httpapi — REST API сервиса заказов.
// В капстоуне это orders/internal/httpapi (контракт «Неделя 5»):
// POST /v1/orders, GET /v1/orders/{id}, GET /v1/orders.
package httpapi

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"practice/domain"
	"practice/service"
)

// Service — интерфейс сервисного слоя, на который опираются хэндлеры.
// В капстоуне его реализует *service.OrderService.
type Service interface {
	CreateOrder(ctx context.Context, customerID string, items []domain.OrderItem) (*domain.Order, error)
	GetOrder(ctx context.Context, id string) (*domain.Order, error)
	ListOrders(ctx context.Context, f service.ListFilter) ([]*domain.Order, error)
}

type api struct {
	svc Service
}

// NewRouter собирает HTTP-роутер сервиса заказов: маршруты и цепочку middleware.
//
// TODO (лекция, разделы «Роутер» и «Цепочки middleware»):
//  1. зарегистрируй маршруты:
//     r.Route("/v1/orders", func(r chi.Router) {
//     r.Post("/", a.handleCreateOrder) // POST /v1/orders
//     r.Get("/", a.handleListOrders)   // GET  /v1/orders
//     r.Get("/{id}", a.handleGetOrder) // GET  /v1/orders/{id}
//     })
//  2. собери middleware-цепочку через Wrap: первый аргумент списка — внешний слой.
//     Подумай над порядком (тест TestMiddlewareChainOrderAndPanic подскажет):
//     Wrap(r, RequestID, Recover, Logging(slog.Default()))
//
// Сейчас маршруты не зарегистрированы и middleware не подключены —
// любой запрос получает 404.
func NewRouter(svc Service) http.Handler {
	r := chi.NewRouter()
	// TODO: маршруты и middleware — см. комментарий выше.
	_ = svc // TODO: хэндлеры задействуют сервис в уроке 10
	return r
}

// handleCreateOrder — POST /v1/orders.
// На этом уроке — заглушка: полный контракт (декодирование, DTO, коды ошибок)
// реализуется в уроке 10 поверх пакета api. Сейчас достаточно вернуть 201
// и пустой JSON-объект; svc пока не используется.
func (a *api) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{}`)
}

// handleGetOrder — GET /v1/orders/{id}. Заглушка до урока 10.
func (a *api) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{}`)
}

// handleListOrders — GET /v1/orders. Заглушка до урока 10.
func (a *api) handleListOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `[]`)
}
