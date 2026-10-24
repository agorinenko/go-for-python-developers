package reportgen_test

import (
	"testing"

	"practice/reportgen"
)

// Эти тесты — контракт поведения Generate: они зелёные с самого начала и
// ДОЛЖНЫ остаться зелёными после оптимизации.

func TestGenerate(t *testing.T) {
	orders := []reportgen.Order{
		{
			ID:         "o-1",
			CustomerID: "c-1",
			Date:       "2026-10-01",
			Lines: []reportgen.Line{
				{ProductID: "p-1", Name: "Книга", Quantity: 2, Price: 10_000},
				{ProductID: "p-2", Name: "Ручка", Quantity: 1, Price: 250},
			},
		},
		{
			ID:         "o-2",
			CustomerID: "c-2",
			Date:       "2026-10-02",
			Lines:      []reportgen.Line{{ProductID: "p-3", Name: "Тетрадь", Quantity: 10, Price: 500}},
		},
		{
			ID:         "o-3",
			CustomerID: "c-1",
			Date:       "2026-10-03",
			Lines:      []reportgen.Line{{ProductID: "p-1", Name: "Книга", Quantity: 1, Price: 10_000}},
		},
	}

	want := "customer c-1\n" +
		"  order o-1 2026-10-01\n" +
		"    Книга x2 100.00\n" +
		"    Ручка x1 2.50\n" +
		"  order o-3 2026-10-03\n" +
		"    Книга x1 100.00\n" +
		"  total: 302.50\n" +
		"customer c-2\n" +
		"  order o-2 2026-10-02\n" +
		"    Тетрадь x10 5.00\n" +
		"  total: 50.00\n" +
		"grand total: 352.50\n"

	if got := string(reportgen.Generate(orders)); got != want {
		t.Fatalf("Generate:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateEmpty(t *testing.T) {
	want := "grand total: 0.00\n"
	if got := string(reportgen.Generate(nil)); got != want {
		t.Fatalf("Generate(nil) = %q, want %q", got, want)
	}
}

func TestGenerateKopecks(t *testing.T) {
	// Деньги — копейки: 333 → "3.33", 1 → "0.01"; форматирование не должно
	// зависеть от способа деления (float или целочисленно).
	orders := []reportgen.Order{
		{
			ID:         "o-1",
			CustomerID: "c-1",
			Date:       "2026-10-04",
			Lines: []reportgen.Line{
				{ProductID: "p-1", Name: "Кофе", Quantity: 1, Price: 333},
				{ProductID: "p-2", Name: "Жвачка", Quantity: 1, Price: 1},
			},
		},
	}
	want := "customer c-1\n" +
		"  order o-1 2026-10-04\n" +
		"    Кофе x1 3.33\n" +
		"    Жвачка x1 0.01\n" +
		"  total: 3.34\n" +
		"grand total: 3.34\n"

	if got := string(reportgen.Generate(orders)); got != want {
		t.Fatalf("Generate:\n got:\n%s\nwant:\n%s", got, want)
	}
}
