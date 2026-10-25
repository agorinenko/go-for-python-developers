# Практика. Урок 18: бенчмарки и профилирование

В пакете `reportgen` — **готовая, намеренно медленная** реализация генератора
отчётов по заказам. Она корректна: тесты зелёные с самого начала и должны
остаться зелёными. Ваша задача — не «починить», а пройти полный цикл:
измерить → профилировать → оптимизировать → измерить снова.

## Задание

1. **Напишите бенчмарки** в `reportgen/bench_test.go` (файл создаёте сами).
   Пример каркаса:

   ```go
   package reportgen_test

   import (
       "fmt"
       "testing"

       "practice/reportgen"
   )

   // makeOrders строит датасет: n клиентов × 2 заказа × 3 позиции.
   func makeOrders(n int) []reportgen.Order {
       orders := make([]reportgen.Order, 0, n*2)
       for i := 0; i < n; i++ {
           for j := 0; j < 2; j++ {
               o := reportgen.Order{
                   ID:         fmt.Sprintf("o-%d-%d", i, j),
                   CustomerID: fmt.Sprintf("c-%d", i),
                   Date:       "2026-10-04",
               }
               for k := 0; k < 3; k++ {
                   o.Lines = append(o.Lines, reportgen.Line{
                       ProductID: fmt.Sprintf("p-%d", k),
                       Name:      fmt.Sprintf("Товар %d", k),
                       Quantity:  k + 1,
                       Price:     int64((k + 1) * 100),
                   })
               }
               orders = append(orders, o)
           }
       }
       return orders
   }

   func BenchmarkGenerate10(b *testing.B)    { benchGenerate(b, 10) }
   func BenchmarkGenerate100(b *testing.B)   { benchGenerate(b, 100) }
   func BenchmarkGenerate1000(b *testing.B)  { benchGenerate(b, 1000) }

   func benchGenerate(b *testing.B, n int) {
       orders := makeOrders(n)
       b.ReportAllocs()
       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           out := reportgen.Generate(orders)
           _ = out
       }
   }
   ```

2. **Снимите профили до оптимизации** (лекция, раздел «pprof»):

   ```sh
   go test -run '^$' -bench . -benchmem -count 6 | tee before.txt
   go test -run '^$' -bench BenchmarkGenerate1000 -cpuprofile cpu.out -memprofile mem.out
   go tool pprof -top reportgen.test cpu.out
   go tool pprof -top -alloc_objects reportgen.test mem.out
   ```

   Разберитесь, какие функции в топе и почему. Занесите находки в
   `REPORT.md`.

3. **Оптимизируйте 2–3 узких места** в `reportgen/reportgen.go`, не меняя
   поведения (тесты — критерий). Кандидаты, видимые в коде: вложенный цикл
   O(n²), повторный `time.Parse`/`Format` одной даты, конкатенация `+=`,
   `fmt.Sprintf` и float-деньги (урок 17: `bytes.Buffer`, `strconv.Append*`,
   целочисленное форматирование копеек). Не обязательно трогать всё —
   выберите то, что подтвердил профиль.

4. **Измерьте снова и заполните `REPORT.md`**:

   ```sh
   go test -run '^$' -bench . -benchmem -count 6 | tee after.txt
   benchstat before.txt after.txt
   ```

   `benchstat` — отдельная утилита: `go install golang.org/x/perf/cmd/benchstat@latest`.
   Если benchstat недоступен — допустимо сравнить строки `BenchmarkGenerate...`
   из `before.txt`/`after.txt` вручную.

## Критерий сдачи

- `go vet ./...` молчит;
- `go test ./...` зелёный — тесты корректности остались зелёными после
  оптимизации;
- `REPORT.md` заполнен: таблица до/после, находки профилировщика (вывод
  `pprof -top`), список изменений, команды, выводы. Эталонных цифр нет —
  главное процесс.

## Перенос в итоговый проект

1. Добавьте бенчмарки горячего пути `orders` (сериализация DTO
   `internal/httpapi`, валидация `internal/service`) — по образцу
   `bench_test.go`.
2. Поднимите `net/http/pprof` в `cmd/orders` (или отдельный отладочный порт),
   снимите CPU-профиль под нагрузкой (`go tool pprof -http :6060` по
   `debug/pprof/profile`), найдите 1–2 горячих места и оптимизируйте их
   (урок 17).
3. Занесите цифры до/после в `orders/README.md` — как в `REPORT.md` этой
   практики.
4. Нагрузочный инструмент на выбор: `k6`, `vegeta` или `oha` — сценарий
   `POST /v1/orders` на 30–60 секунд, до и после; сравните p95.
