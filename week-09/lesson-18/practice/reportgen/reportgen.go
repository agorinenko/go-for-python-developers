// Package reportgen — генератор текстовых отчётов по заказам.
//
// Реализация НАМЕРЕННО медленная — это объект оптимизации практики урока 18
// (см. README.md и REPORT.md). Поведение зафиксировано тестами в
// reportgen_test.go и должно остаться неизменным: входные даты считаются
// валидными ("2006-01-02"), деньги — int64 копеек.
package reportgen

import (
	"fmt"
	"time"
)

// Order — заказ из входных данных отчёта.
type Order struct {
	ID         string
	CustomerID string
	Date       string // "2006-01-02" — дата заказа
	Lines      []Line
}

// Line — позиция заказа.
type Line struct {
	ProductID string
	Name      string
	Quantity  int
	Price     int64 // в копейках
}

// Total — сумма заказа в копейках.
func (o Order) Total() int64 {
	var t int64
	for _, l := range o.Lines {
		t += int64(l.Quantity) * l.Price
	}
	return t
}

// Generate строит отчёт: секция на каждого клиента (в порядке первого
// появления), внутри — заказы со строками позиций и итогом, в конце —
// общий итог. Формат см. в reportgen_test.go.
//
// Узкие места (видны в CPU- и alloc-профиле):
//  1. вложенный цикл по ВСЕМ заказам на каждого клиента — O(n²);
//  2. повторный time.Parse + Format одной и той же даты в цикле;
//  3. конкатенация строк через `+=` (квадратичное копирование);
//  4. fmt.Sprintf и float-арифметика для денег;
//  5. повторный полный проход ради суммы клиента.
func Generate(orders []Order) []byte {
	var out string
	seen := map[string]bool{}
	for _, o := range orders {
		if seen[o.CustomerID] {
			continue
		}
		seen[o.CustomerID] = true
		out += "customer " + o.CustomerID + "\n"

		for _, o2 := range orders {
			if o2.CustomerID != o.CustomerID {
				continue
			}
			date, err := time.Parse("2006-01-02", o2.Date)
			if err != nil {
				continue
			}
			out += fmt.Sprintf("  order %s %s\n", o2.ID, date.Format("2006-01-02"))
			for _, l := range o2.Lines {
				out += fmt.Sprintf("    %s x%d %.2f\n", l.Name, l.Quantity, float64(l.Price)/100)
			}
		}

		var total int64
		for _, o2 := range orders {
			if o2.CustomerID != o.CustomerID {
				continue
			}
			total += o2.Total()
		}
		out += fmt.Sprintf("  total: %.2f\n", float64(total)/100)
	}

	var grand int64
	for _, o := range orders {
		grand += o.Total()
	}
	out += fmt.Sprintf("grand total: %.2f\n", float64(grand)/100)
	return []byte(out)
}
