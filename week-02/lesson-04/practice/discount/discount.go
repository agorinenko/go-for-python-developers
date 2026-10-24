// Package discount — расчёт скидок: правила по категориям товаров и порогам
// суммы плюс итоговый отчёт заказа.
//
// ВНИМАНИЕ: код пакета ГОТОВЫЙ и намеренно не содержит TODO. Задание этой
// практики — написать тесты к нему (см. README.md и lecture.md): table-driven,
// testify-ассерты, golden file, граничные случаи, покрытие ≥ 90%.
package discount

import (
	"fmt"
	"strings"
)

// Category — категория товара.
type Category string

const (
	CategoryBooks       Category = "books"
	CategoryElectronics Category = "electronics"
	CategoryClothes     Category = "clothes"
	CategoryOther       Category = "other"
)

// Item — товарная позиция; Price в копейках (никогда не float64).
type Item struct {
	Name     string
	Category Category
	Price    int64 // в копейках
}

// Rule — правило скидки: Percent процентов на категорию, если сумма позиций
// категории не меньше Threshold (копейки). Percent вне 0..100 прижимается
// к границам.
type Rule struct {
	Category  Category
	Percent   int   // 0..100, вне диапазона — прижимается
	Threshold int64 // в копейках; 0 — правило действует на любую сумму
}

// CategoryTotal — сумма цен позиций заданной категории.
func CategoryTotal(items []Item, category Category) int64 {
	var total int64
	for _, it := range items {
		if it.Category == category {
			total += it.Price
		}
	}
	return total
}

// DiscountFor — размер скидки по одному правилу.
// Округление вниз до копейки (целочисленное деление).
func DiscountFor(subtotal int64, r Rule) int64 {
	if subtotal <= 0 || subtotal < r.Threshold {
		return 0
	}
	percent := r.Percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return subtotal * int64(percent) / 100
}

// ApplyRules — суммарная скидка по позициям. Для каждой категории из
// подходящих правил (subtotal >= Threshold) выбирается правило с НАИБОЛЬШИМ
// порогом; при равенстве порогов — с наибольшим процентом. Правила на
// категорию не суммируются: действует максимум одно.
func ApplyRules(items []Item, rules []Rule) int64 {
	categories := []Category{CategoryBooks, CategoryElectronics, CategoryClothes, CategoryOther}
	var total int64
	for _, cat := range categories {
		subtotal := CategoryTotal(items, cat)
		var best Rule
		for _, r := range rules {
			if r.Category != cat {
				continue
			}
			if subtotal < r.Threshold {
				continue
			}
			if r.Threshold > best.Threshold ||
				(r.Threshold == best.Threshold && r.Percent > best.Percent) {
				best = r
			}
		}
		total += DiscountFor(subtotal, best)
	}
	return total
}

// Bill — итоговый отчёт о заказе.
type Bill struct {
	Items    []Item
	Subtotal int64 // копейки
	Discount int64 // копейки
	Total    int64 // копейки
}

// MakeBill собирает Bill по позициям и правилам.
func MakeBill(items []Item, rules []Rule) Bill {
	var subtotal int64
	for _, it := range items {
		subtotal += it.Price
	}
	d := ApplyRules(items, rules)
	return Bill{Items: items, Subtotal: subtotal, Discount: d, Total: subtotal - d}
}

// FormatBill — стабильное текстовое представление отчёта. Формат
// детерминирован (не зависит от порядка карт), поэтому подходит для
// golden-файлов.
func FormatBill(b Bill) string {
	var sb strings.Builder
	sb.WriteString("Order report\n")
	fmt.Fprintf(&sb, "- items: %d\n", len(b.Items))
	for _, it := range b.Items {
		fmt.Fprintf(&sb, "  - %s (%s): %s\n", it.Name, it.Category, FormatKopecks(it.Price))
	}
	fmt.Fprintf(&sb, "- subtotal: %s\n", FormatKopecks(b.Subtotal))
	fmt.Fprintf(&sb, "- discount: %s\n", FormatKopecks(b.Discount))
	fmt.Fprintf(&sb, "- total: %s\n", FormatKopecks(b.Total))
	return sb.String()
}

// FormatKopecks — int64 копеек в человекочитаемый вид: "12.34 RUB".
// Отрицательные суммы выводятся со знаком минус.
func FormatKopecks(k int64) string {
	sign := ""
	if k < 0 {
		sign = "-"
		k = -k
	}
	return fmt.Sprintf("%s%d.%02d RUB", sign, k/100, k%100)
}
