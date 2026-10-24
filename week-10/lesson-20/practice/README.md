# Практика. Урок 20: метрики и трейсинг

Пакет `metrics` — RED-метрики HTTP API капстоуна (CAPSTONE.md, неделя 10:
`internal/telemetry/metrics` + эндпоинт `/metrics`):

- `orders_http_requests_total` — counter, метки `method`, `status`;
- `orders_http_request_duration_seconds` — histogram с API-бакетами
  (5 мс … 10 с), метки `method`, `status`;
- `orders_http_in_flight_requests` — gauge выполняющихся запросов.

## Задание

Реализуйте `TODO` в `metrics/metrics.go`:

1. `statusRecorder.WriteHeader` / `statusRecorder.Write` — перехват кода
   ответа: явный `WriteHeader(code)` запоминает код; `Write` без
   `WriteHeader` означает 200. Оба метода делегируют базовому
   `http.ResponseWriter` и не должны менять ответ.
2. Тело middleware в `NewMiddleware`:
   - `inFlight.Inc()` в начале, `Dec()` через `defer`;
   - `defer`: наблюдение длительности `time.Since(start).Seconds()` в
     гистограмму с метками `r.Method` и кодом ответа;
   - обернуть `w` в `statusRecorder` и передать дальше;
   - после `next.ServeHTTP` — инкремент счётчика с метками `r.Method` и
     `strconv.Itoa(rec.code)`.
   Не забудьте импорты `time` и `strconv`.

Метрики уже зарегистрированы в `NewMiddleware` — менять их имена, метки и
бакеты нельзя (на них опираются тесты и дашборды).

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное.

## Перенос в капстоун

1. Скопируйте `metrics/metrics.go` в `orders/internal/telemetry/metrics/`
   (пакет `metrics`).
2. В `cmd/orders/main.go`: `reg := prometheus.NewRegistry()`,
   `mw := metrics.NewMiddleware(reg)` — подключите к роутеру; добавьте
   эндпоинт `/metrics`:

   ```go
   mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
   ```

3. Обратите внимание на порядок middleware: метрики должны считать
   **все** запросы, поэтому обёртка метрик — самая внешняя (поверх
   логирования и rate limiting, если они меняют ответ).
4. Проверьте вручную: `curl localhost:8080/metrics | grep orders_http` —
   счётчики растут, после простоя gauge равен 0.
5. На следующем шаге (по желанию) — трейсинг: OTel-инструментовка
   `otelhttp` ставится внутрь метрик; связка «лог ↔ трейс» — через
   `trace_id`/`span_id` в атрибутах логгера урока 19.
