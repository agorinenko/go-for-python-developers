package optimize_test

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"practice/optimize"
)

// ────────────────────────────── эталоны поведения ──────────────────────────────
// Наивные реализации из optimize.go, продублированные здесь: они определяют
// ПОВЕДЕНИЕ, а не производительность. Ваша оптимизированная версия обязана
// совпадать с эталоном на всех входах, включая пустые и Unicode.

func refConcat(parts []string) string {
	var s string
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}

func refBuildReport(orders []optimize.Order) []byte {
	var out []byte
	for _, o := range orders {
		line := o.ID + ": " + strconv.FormatInt(o.Total, 10) + "\n"
		out = append(out, []byte(line)...)
	}
	return out
}

func refParseIDs(csv string) []string {
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

// isEmptySlice — true для слайса нулевой длины (nil или пустой).
func isEmptySlice(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Slice && rv.Len() == 0
}

// deepEqualTolerant — DeepEqual, считающий nil-слайс равным пустому:
// разница nil/пустой в этой практике поведением не является.
func deepEqualTolerant(got, want any) bool {
	if isEmptySlice(got) && isEmptySlice(want) {
		return true
	}
	return reflect.DeepEqual(got, want)
}

// ──────────────────────────────── корректность ────────────────────────────────

func TestConcat(t *testing.T) {
	cases := []struct {
		name  string
		parts []string
	}{
		{"empty", nil},
		{"single", []string{"a"}},
		{"many", []string{"a", "b", "c"}},
		{"unicode", []string{"один", "два", "три"}},
		{"separators inside", []string{"a, b", "c"}},
		{"empty element", []string{"", "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := refConcat(tc.parts)
			if got := optimize.Concat(tc.parts); got != want {
				t.Fatalf("Concat(%q) = %q, want %q", tc.parts, got, want)
			}
		})
	}
}

func TestBuildReport(t *testing.T) {
	cases := []struct {
		name   string
		orders []optimize.Order
	}{
		{"empty", nil},
		{"single", []optimize.Order{{ID: "o-1", Total: 12_345}}},
		{"many", []optimize.Order{
			{ID: "o-1", Total: 100},
			{ID: "o-2", Total: 0},
			{ID: "o-3", Total: 9_999_999},
		}},
		{"unicode id", []optimize.Order{{ID: "заказ-1", Total: 50}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := refBuildReport(tc.orders)
			got := optimize.BuildReport(tc.orders)
			if !bytes.Equal(got, want) {
				t.Fatalf("BuildReport(%+v) = %q, want %q", tc.orders, got, want)
			}
		})
	}
}

func TestParseIDs(t *testing.T) {
	cases := []struct {
		name string
		csv  string
	}{
		{"empty", ""},
		{"one", "a"},
		{"many", "a,b,c"},
		{"spaces", " a , b ,c "},
		{"empty fields", "a,,b,"},
		{"only separators", ",,,"},
		{"unicode", "id-α,id-β, id-γ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := refParseIDs(tc.csv)
			got := optimize.ParseIDs(tc.csv)
			if !deepEqualTolerant(got, want) {
				t.Fatalf("ParseIDs(%q) = %q, want %q", tc.csv, got, want)
			}
		})
	}
}

// ───────────────────────────── пороги аллокаций ───────────────────────────────
// Измерено на эталонной оптимизации (1–2 аллокации/вызов); пороги даны с
// запасом. Наивные реализации дают тысячи — такие тесты не пройдут.

func TestConcatAllocs(t *testing.T) {
	parts := make([]string, 1000)
	for i := range parts {
		parts[i] = strconv.Itoa(i)
	}
	allocs := testing.AllocsPerRun(100, func() {
		got := optimize.Concat(parts)
		_ = got
	})
	if allocs > 3 {
		t.Fatalf("Concat: %v аллокаций на 1000 частей, порог 3 (эталон: 1)", allocs)
	}
}

func TestBuildReportAllocs(t *testing.T) {
	orders := make([]optimize.Order, 1000)
	for i := range orders {
		orders[i] = optimize.Order{ID: "order-" + strconv.Itoa(i), Total: int64(i * 100)}
	}
	allocs := testing.AllocsPerRun(100, func() {
		got := optimize.BuildReport(orders)
		_ = got
	})
	if allocs > 5 {
		t.Fatalf("BuildReport: %v аллокаций на 1000 заказов, порог 5 (эталон: 2)", allocs)
	}
}

func TestParseIDsAllocs(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("id-")
		b.WriteString(strconv.Itoa(i))
	}
	csv := b.String()

	allocs := testing.AllocsPerRun(100, func() {
		got := optimize.ParseIDs(csv)
		_ = got
	})
	if allocs > 3 {
		t.Fatalf("ParseIDs: %v аллокаций на 1000 элементов, порог 3 (эталон: 1)", allocs)
	}
}
