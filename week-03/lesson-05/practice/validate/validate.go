// Package validate — параллельная валидация позиций заказа.
// Единственный акцент урока: конкурентность с сохранением порядка результата.
package validate

import (
	"errors"
)

// Item — одна позиция заказа.
type Item struct {
	ID       string
	Quantity int
	Price    int64 // в копейках
}

// Sentinel-ошибки валидации позиции.
var (
	ErrEmptyID     = errors.New("validate: product id is empty")
	ErrBadQuantity = errors.New("validate: quantity must be > 0")
	ErrBadPrice    = errors.New("validate: price must be >= 0")
)

// Result — результат проверки одной позиции.
// Index — индекс позиции во входном слайсе: результаты идут СТРОГО в порядке
// входных позиций, Index делает это явным.
type Result struct {
	Index int
	Item  Item
	Err   error // nil — позиция валидна
}

// validateItem — готовая функция проверки одной позиции (её менять не нужно).
// В капстоуне на её месте будет domain.OrderValidator.Validate (неделя 2).
func validateItem(item Item) error {
	if item.ID == "" {
		return ErrEmptyID
	}
	if item.Quantity <= 0 {
		return ErrBadQuantity
	}
	if item.Price < 0 {
		return ErrBadPrice
	}
	return nil
}

// ValidateItems валидирует все позиции параллельно, но возвращает результаты
// СТРОГО в порядке входного слайса: results[i] соответствует items[i].
//
// workers — предел одновременно работающих горутин; workers < 1 — используется
// runtime.GOMAXPROCS(0).
//
// TODO: реализуй:
//  1. выдели results := make([]Result, len(items)) ЗАРАНЕЕ — писать в
//     results[i] из разных горутин безопасно: каждая пишет ТОЛЬКО в свой
//     элемент (никакой общей мутации, sync не нужен);
//  2. организуй ограничение конкурентности на выбор:
//     а) N воркеров (N = workers или GOMAXPROCS) с общим каналом заданий,
//     где задание — индекс позиции: for i := range jobs { results[i] = ... };
//     б) семафор на канале struct{} перед запуском горутины на позицию;
//  3. каждая позиция валидируется ровно один раз вызовом validateItem;
//  4. дождись завершения всех горутин (sync.WaitGroup) и верни results.
func ValidateItems(items []Item, workers int) []Result {
	// TODO: реализуй. Заглушка: пустой результат — тесты падают на длине.
	return nil
}
