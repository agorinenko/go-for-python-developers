// Package refactor — учебный пакет с неидиоматичным кодом.
// Задание: сделайте код идиоматичным и исправьте ошибки поведения,
// не меняя сигнатуры публичных функций (тесты должны остаться зелёными).
package refactor

import (
	"fmt"
	"strconv"
)

// Item — позиция отчёта.
type Item struct {
	Name  string
	Price float64
	Count int
}

// FormatReport возвращает построчный отчёт о позициях.
//
// TODO: конкатенация строк в цикле квадратична по времени.
// Перепиши на strings.Join (лекция, раздел «Подводные камни»).
func FormatReport(items []Item) string {
	result := ""
	for i := range items {
		result = result + fmt.Sprintf("%s: %.2f x %d\n", items[i].Name, items[i].Price, items[i].Count)
	}
	return result
}

// ParseQuantity разбирает количество из строки.
//
// TODO: ошибка молча проглатывается — невалидная строка неотличима от «0».
// Верни ошибку вызывающему коду: измени сигнатуру на (int, error)
// и обработай ошибку там, где функция вызывается.
func ParseQuantity(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

// SumQuantities возвращает сумму количеств.
//
// TODO: сейчас функция обнуляет весь итог, если хоть одна строка невалидна.
// Ожидаемое поведение (см. тесты): невалидные строки пропускаются,
// валидные суммируются.
func SumQuantities(rows []string) int {
	total := 0
	for _, row := range rows {
		n, err := strconv.Atoi(row)
		if err != nil {
			total = total + 0
		}
		total = total + n
	}
	return total
}

// ProcessItems агрегирует стоимость по именам позиций.
//
// TODO: три проблемы (см. тесты):
//  1. функция мутирует входной слайс (сортировкой) — вызывающий код этого не ожидает;
//  2. стоимость считается как Price, а не Price*Count;
//  3. map[string]interface{} там, где достаточно map[string]float64.
func ProcessItems(items []Item) map[string]float64 {
	// мутируем вход — так делать нельзя
	//sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	data := make(map[string]float64)
	for _, it := range items {
		var current float64
		if v, ok := data[it.Name]; ok {
			current = v
		}
		data[it.Name] = current + it.Price*float64(it.Count)
	}

	result := make(map[string]float64, len(data))
	for name, v := range data {
		result[name] = v
	}
	return result
}
