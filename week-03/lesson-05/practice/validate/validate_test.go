package validate_test

import (
	"errors"
	"reflect"
	"testing"

	"practice/validate"
)

// mixedItems — позиции с разными исходами валидации (по одному дефекту на позицию).
func mixedItems() []validate.Item {
	return []validate.Item{
		{ID: "p-1", Quantity: 2, Price: 100},   // валидна
		{ID: "", Quantity: 1, Price: 10},       // ErrEmptyID
		{ID: "p-3", Quantity: 0, Price: 10},    // ErrBadQuantity
		{ID: "p-4", Quantity: 1, Price: -5},    // ErrBadPrice
		{ID: "p-5", Quantity: 3, Price: 1_000}, // валидна
	}
}

// wantResults строит эталон: та же логика, что у validateItem, но описанная
// в тесте независимо (тест — спецификация поведения).
func wantResults(items []validate.Item) []validate.Result {
	out := make([]validate.Result, len(items))
	for i, it := range items {
		out[i] = validate.Result{Index: i, Item: it}
		switch {
		case it.ID == "":
			out[i].Err = validate.ErrEmptyID
		case it.Quantity <= 0:
			out[i].Err = validate.ErrBadQuantity
		case it.Price < 0:
			out[i].Err = validate.ErrBadPrice
		}
	}
	return out
}

func TestValidateItemsResultsInOrder(t *testing.T) {
	items := mixedItems()
	got := validate.ValidateItems(items, 2)

	if len(got) != len(items) {
		t.Fatalf("ValidateItems вернул %d результатов, want %d", len(got), len(items))
	}
	for i := range items {
		if got[i].Index != i {
			t.Errorf("результат %d: Index = %d, want %d", i, got[i].Index, i)
		}
		if got[i].Item != items[i] {
			t.Errorf("результат %d: Item = %+v, want %+v", i, got[i].Item, items[i])
		}
	}
	wantErrs := []error{nil, validate.ErrEmptyID, validate.ErrBadQuantity, validate.ErrBadPrice, nil}
	for i, want := range wantErrs {
		switch {
		case want == nil && got[i].Err != nil:
			t.Errorf("позиция %d: err = %v, want nil", i, got[i].Err)
		case want != nil && !errors.Is(got[i].Err, want):
			t.Errorf("позиция %d: err = %v, want errors.Is %v", i, got[i].Err, want)
		}
	}
}

func TestValidateItemsWorkerCountIndependent(t *testing.T) {
	items := mixedItems()
	want := wantResults(items)

	// Результат не должен зависеть от числа воркеров — это важно:
	// конкурентность не имеет права менять семантику функции.
	for _, workers := range []int{1, 2, 3, 7, len(items), 0} {
		got := validate.ValidateItems(items, workers)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("workers=%d: результат отличается от эталона:\n got: %v\nwant: %v", workers, got, want)
		}
	}
}

func TestValidateItemsLarge(t *testing.T) {
	const n = 10_000
	items := make([]validate.Item, n)
	for i := range items {
		items[i] = validate.Item{ID: "p", Quantity: 1, Price: 10}
		if i%7 == 0 {
			items[i].ID = "" // каждая седьмая позиция невалидна
		}
	}
	want := wantResults(items)

	got := validate.ValidateItems(items, 8)
	if len(got) != n {
		t.Fatalf("ValidateItems обработал %d позиций из %d", len(got), n)
	}
	for i := range got {
		if got[i].Index != i || got[i].Item != items[i] {
			t.Fatalf("позиция %d не на своём месте: Index=%d Item=%+v", i, got[i].Index, got[i].Item)
		}
	}
	for i := range got {
		// у каждого результата — корректная ошибка относительно эталона
		switch {
		case want[i].Err == nil && got[i].Err != nil:
			t.Fatalf("позиция %d: err = %v, want nil", i, got[i].Err)
		case want[i].Err != nil && !errors.Is(got[i].Err, want[i].Err):
			t.Fatalf("позиция %d: err = %v, want errors.Is %v", i, got[i].Err, want[i].Err)
		}
	}
}

func TestValidateItemsEmpty(t *testing.T) {
	if got := validate.ValidateItems(nil, 4); len(got) != 0 {
		t.Fatalf("ValidateItems(nil) = %v, want пустой результат", got)
	}
}
