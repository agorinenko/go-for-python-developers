// Package optimize — горячий путь сервиса заказов: сериализация, отчёты,
// разбор входных данных. Задание: переписать наивные реализации с минимумом
// аллокаций, не меняя сигнатуры и поведение (эталон поведения — в
// optimize_test.go; пороги аллокаций — в *_test.go).
package optimize

import (
	"bytes"
	"strconv"
	"strings"
)

// Order — минимальная копия заказа для практики.
// Каноническая модель — в капстоуне (orders/internal/domain).
type Order struct {
	ID    string
	Total int64 // в копейках
}

// Concat соединяет части строки через ", ".
//
// Наивная реализация ниже — корректная, но каждый `+=` аллоцирует новую
// строку и копирует всё накопленное: O(n²) по суммарной длине.
//
// TODO: перепиши с минимумом аллокаций, поведение то же
// (лекция, разделы «strings.Builder» и «Рост слайсов»):
// подсказка — strings.Join. Сигнатуру и поведение менять нельзя:
// пустой вход → "", один элемент → без разделителя.
func Concat(parts []string) string {
	var s string
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}

// BuildReport строит отчёт: по строке "ID: total\n" на каждый заказ.
//
// Наивная реализация: string-конкатенация + конверсия []byte(line) на
// каждый заказ — десятки тысяч аллокаций на 1000 заказов.
//
// TODO: перепиши с минимумом аллокаций, поведение то же
// (лекция, разделы «Строки и []byte» и «sync.Pool»):
// подсказки — bytes.Buffer с предварительным Grow, WriteString,
// strconv.AppendInt в стековый буфер [32]byte (никаких конверсий
// string ↔ []byte в цикле). Пустой вход → пустой результат.
func BuildReport(orders []Order) []byte {
	var out []byte
	for _, o := range orders {
		line := o.ID + ": " + strconv.FormatInt(o.Total, 10) + "\n"
		out = append(out, []byte(line)...)
	}
	return out
}

// ParseIDs разбирает CSV-строку в список непустых id; пробелы вокруг
// элементов обрезаются, пустые элементы пропускаются.
//
// Наивная реализация: []byte(csv) копирует ВСЮ строку, bytes.Split копирует
// каждое поле, string(f) копирует ещё раз — тысячи аллокаций на 1000
// элементов.
//
// TODO: перепиши с минимумом аллокаций, поведение то же
// (лекция, раздел «Подлежащий массив»: под-слайсы строки не аллоцируют):
// подсказки — strings.IndexByte для поиска запятых, strings.Count для
// предварительной ёмкости слайса, strings.TrimSpace. Не превращай поля
// в копии без необходимости.
func ParseIDs(csv string) []string {
	fields := bytes.Split([]byte(csv), []byte(","))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		s := strings.TrimSpace(string(f))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
