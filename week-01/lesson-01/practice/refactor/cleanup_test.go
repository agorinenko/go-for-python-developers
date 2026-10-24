package refactor_test

import (
	"reflect"
	"testing"

	"practice/refactor"
)

func TestFormatReport(t *testing.T) {
	items := []refactor.Item{
		{Name: "Книга", Price: 10.5, Count: 2},
		{Name: "Ручка", Price: 1.25, Count: 5},
	}
	want := "Книга: 10.50 x 2\nРучка: 1.25 x 5\n"
	if got := refactor.FormatReport(items); got != want {
		t.Fatalf("FormatReport() = %q, want %q", got, want)
	}
}

func TestSumQuantities(t *testing.T) {
	cases := []struct {
		name string
		rows []string
		want int
	}{
		{"empty", nil, 0},
		{"all valid", []string{"1", "2", "3"}, 6},
		{"invalid rows skipped", []string{"5", "abc", "3"}, 8},
		{"all invalid", []string{"abc", "def"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refactor.SumQuantities(tc.rows); got != tc.want {
				t.Fatalf("SumQuantities(%v) = %d, want %d", tc.rows, got, tc.want)
			}
		})
	}
}

func TestProcessItems(t *testing.T) {
	items := []refactor.Item{
		{Name: "B", Price: 5, Count: 3},
		{Name: "A", Price: 10, Count: 2},
	}
	original := append([]refactor.Item(nil), items...)

	got := refactor.ProcessItems(items)

	want := map[string]float64{"A": 20, "B": 15} // Price*Count, а не Price
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProcessItems() = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(items, original) {
		t.Fatalf("ProcessItems mutated the input: %v → %v", original, items)
	}
}
