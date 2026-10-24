// Package service — сервисный слой: бизнес-логика заказов ПОВЕРХ интерфейсов.
// Контракт заморожен в CAPSTONE.md (неделя 2): сигнатуры менять нельзя.
//
// Ключевая идея урока: OrderService зависит от domain.OrderRepository и
// domain.OrderValidator — интерфейсов, а не конкретных реализаций. В тестах
// это фейки в памяти, в капстоуне — PostgreSQL (неделя 6).
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"practice/domain"
)

// newID генерирует уникальный ID заказа (16 байт crypto/rand в hex).
// Готовый помощник — акцент урока на интерфейсах, а не на генерации ID.
var newID = func() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read не ошибается на практике
	return hex.EncodeToString(b)
}

// OrderService — сервис заказов. Все зависимости — интерфейсы, поэтому
// сервис ничего не знает об их реализациях.
type OrderService struct {
	repo      domain.OrderRepository
	validator domain.OrderValidator
}

// NewOrderService собирает сервис из зависимостей.
func NewOrderService(repo domain.OrderRepository, validator domain.OrderValidator) *OrderService {
	return &OrderService{repo: repo, validator: validator}
}

// CreateOrder создаёт заказ в статусе StatusNew.
//
// TODO: реализуй:
//  1. пустой customerID -> domain.ErrEmptyCustomer;
//  2. пустой items -> domain.ErrNoItems;
//  3. провалидируй КАЖДУЮ позицию через s.validator.Validate(ctx, item)
//     (не забывай ctx!); при ошибке — верни её, обернув с деталями так,
//     чтобы errors.Is находил И ErrInvalidItem, И исходную ошибку
//     (Go 1.20+ поддерживает несколько %w в одном fmt.Errorf):
//     fmt.Errorf("%w: product %q: %w", domain.ErrInvalidItem, item.ProductID, err);
//  4. собери domain.Order: ID = newID(), Status = StatusNew,
//     Total = сумма Amount() позиций (int64 копеек!), CreatedAt = time.Now();
//  5. сохрани через s.repo.Save(ctx, o), верни заказ по указателю.
func (s *OrderService) CreateOrder(ctx context.Context, customerID string, items []domain.OrderItem) (*domain.Order, error) {
	// TODO: реализуй. Заглушка: «сырой» заказ без валидации и без
	// сохранения — тесты падают именно на этом.
	return &domain.Order{
		ID:         newID(),
		CustomerID: customerID,
		Items:      items,
		Status:     domain.StatusNew,
		Total:      0,
		CreatedAt:  time.Now(),
	}, nil
}

// GetOrder возвращает заказ по ID. Неизвестный ID — domain.ErrOrderNotFound.
//
// TODO: реализуй: s.repo.GetByID(ctx, id); ошибку репозитория пробрасывай
// как есть (или оборачивай: fmt.Errorf("get order %s: %w", id, err)) —
// errors.Is обязан находить domain.ErrOrderNotFound.
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	// TODO: реализуй. Заглушка.
	return nil, nil
}

// ListOrders возвращает заказы по фильтру. Фильтр передаётся в репозиторий
// как есть — реализация хранилища сама решает, как применять пустые поля.
//
// TODO: реализуй: s.repo.List(ctx, f).
func (s *OrderService) ListOrders(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error) {
	// TODO: реализуй. Заглушка.
	return nil, nil
}

// CancelOrder отменяет заказ: new → cancelled.
//
// TODO: реализуй:
//  1. s.repo.GetByID(ctx, id); не найден — пробрось domain.ErrOrderNotFound;
//  2. переход статуса через o.ApplyStatus(domain.StatusCancelled) —
//     невалидный переход вернёт domain.ErrInvalidTransition;
//  3. сохрани новый статус через s.repo.UpdateStatus(ctx, id, o.Status),
//     ошибку — верни.
func (s *OrderService) CancelOrder(ctx context.Context, id string) error {
	// TODO: реализуй. Заглушка: не-nil ошибка, чтобы тесты падали
	// ассертами, а не паникой на nil-заказе ниже по коду теста.
	return errors.New("service: CancelOrder not implemented")
}
