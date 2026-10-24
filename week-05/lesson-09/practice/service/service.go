// Package service — учебная копия сервисного слоя капстоуна
// (orders/internal/service, неделя 2). Упрощение для автономности практики:
// in-memory хранилище вместо интерфейса OrderRepository. Код готовый.
package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"practice/domain"
)

// ListFilter — фильтр списка заказов. В капстоуне живёт в internal/service.
type ListFilter struct {
	CustomerID string
	Status     domain.OrderStatus
	Limit      int
}

// OrderService — сервис заказов с in-memory хранилищем.
type OrderService struct {
	mu   sync.RWMutex
	byID map[string]*domain.Order
	seq  int64
}

// NewOrderService создаёт пустой сервис.
func NewOrderService() *OrderService {
	return &OrderService{byID: make(map[string]*domain.Order)}
}

// CreateOrder создаёт заказ с новым идентификатором.
func (s *OrderService) CreateOrder(ctx context.Context, customerID string, items []domain.OrderItem) (*domain.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("o-%d", s.seq)
	o, err := domain.NewOrder(id, customerID, items)
	if err != nil {
		return nil, err
	}
	s.byID[id] = o
	return clone(o), nil
}

// GetOrder возвращает заказ по id.
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return clone(o), nil
}

// ListOrders возвращает заказы по фильтру, отсортированные по id.
func (s *OrderService) ListOrders(ctx context.Context, f ListFilter) ([]*domain.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Order, 0, len(s.byID))
	for _, o := range s.byID {
		if f.CustomerID != "" && o.CustomerID != f.CustomerID {
			continue
		}
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		out = append(out, clone(o))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func clone(o *domain.Order) *domain.Order {
	c := *o
	c.Items = append([]domain.OrderItem(nil), o.Items...)
	return &c
}
