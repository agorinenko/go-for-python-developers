# Урок 20. Метрики и трейсинг

## Цели урока

- освоить модель Prometheus: counter/gauge/histogram, метки и ловушку кардинальности;
- поставить RED-метрики (Rate/Errors/Duration) на HTTP-сервис через client_golang и `/metrics`;
- понять, какие метрики нужны сервису заказов, а какие — шум;
- познакомиться с OpenTelemetry: spans, распространение контекста, `otelhttp` — и связать логи, метрики и трейсы.

## Задача из итогового проекта

Неделя 10 итогового проекта — observability: `internal/telemetry/metrics` и эндпоинт
`/metrics` у сервиса. К концу урока у сервиса заказов есть RED-метрики по
API, а в логах (урок 19) — `trace_id`, по которому трейс связывается с
записью лога. Вместе это — «три кита» observability, которыми сервис готов
к эксплуатации под нагрузкой.

## Модель Prometheus

Prometheus — **pull-модель**: сервис экспортирует метрики текстом на
`/metrics`, Prometheus периодически их забирает (scrape). Метрика — это
имя + набор меток + значение + тип:

- **Counter** — только растёт (`http_requests_total`), кроме `Reset()`.
  Производные: rate (запросов/с) — `rate(orders_http_requests_total[5m])`.
- **Gauge** — вверх и вниз (температура, in-flight, размер очереди).
- **Histogram** — распределение значений по **бакетам** (`le="0.1"` и т. д.)
  + сумма + счётчик: даёт p50/p95/p99 (`histogram_quantile`) ценой выбора
  бакетов заранее.
- **Summary** — квантили считает сам клиент: «удобно», но не агрегируется
  по инстансам, поэтому в распределённых системах почти всегда гистограмма.

Метки — измерения: `{method="GET", status="200"}`. **Кардинальность** —
главная ловушка: каждая комбинация меток — отдельный временной ряд в памяти
Prometheus. `user_id`, `order_id`, URL с параметрами в метках — прямой путь
к «Prometheus съел всю память». Правило: метки — только из ограниченного
набора значений (method, status, endpoint-шаблон), уникальные значения —
в трейсы и логи.

```go
// Плохо: кардинальность == число клиентов (и растёт вечно)
requests.WithLabelValues("GET", "/v1/orders/"+orderID).Inc()

// Хорошо: ограниченный набор — «ручка» как константа
requests.WithLabelValues("GET", "/v1/orders/{id}").Inc()
```

## client_golang и /metrics

```go
reg := prometheus.NewRegistry() // свой регистр, не Default — тестируемо и изолировано

requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
    Namespace: "orders", Subsystem: "http", Name: "requests_total",
}, []string{"method", "status"})
requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Namespace: "orders", Subsystem: "http", Name: "request_duration_seconds",
    Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"method", "status"})
inFlight := prometheus.NewGauge(prometheus.GaugeOpts{ /* ... */ })
reg.MustRegister(requestsTotal, requestDuration, inFlight)

mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
```

Бакеты подбираются под целевые латентности API: покрыть диапазон от «быстро»
до «уже плохо» — 5 мс … 10 с для HTTP — разумно. Слишком много бакетов —
сотни рядов на метрику; слишком мало — квантили слепые.

## RED-метрики

RED — Rate, Errors, Duration: минимум, с которого начинается любой сервис.

- **Rate** — сколько запросов: counter `requests_total` по `method`.
- **Errors** — сколько из них с ошибкой: тот же counter с `status` —
  `status="5xx"` (и отдельно 4xx, чтобы отличать «клиент ошибся» от
  «мы падаем»).
- **Duration** — сколько это заняло: histogram `request_duration_seconds`.

Дополнительно в итоговом проекте — `in_flight_requests` (gauge): виден «затык» до
того, как он стал таймаутами. Это не RED, но дёшево и очень полезно при
разборе инцидентов: если in-flight растёт, а duration не падает — сервис
ждёт зависимость.

Middleware-паттерн из практики: обернуть `ResponseWriter` recorder'ом
(перехват статуса: `WriteHeader` или неявные 200 при `Write`), `Inc`/`Dec`
gauge через `defer`, наблюдение длительности через `defer`, инкремент
счётчика после `next`. Порядок middleware: метрики — **самые внешние**,
чтобы считать всё, включая то, что отфильтровал rate limiter.

```go
type statusRecorder struct {
    http.ResponseWriter
    code int
}

func (r *statusRecorder) WriteHeader(code int) {
    if r.code == 0 { r.code = code }
    r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
    if r.code == 0 { r.code = http.StatusOK } // Write без WriteHeader — это 200
    return r.ResponseWriter.Write(b)
}
```

Дальше — дешёвые, но критичные для эксплуатации метрики. Что мерить в
сервисе заказов: время запросов к PostgreSQL (histogram по
`operation="save"|"get"`), размер/латентность публикации в Kafka, число
pending в outbox (gauge — алерт «реле не выгребает»), ошибки gRPC-клиента
catalog. Бизнес-метрики («заказов создано») — это counter в сервисном слое,
а не сумма HTTP-счётчиков: HTTP может считать повторные попытки и повторные запросы.

Пара правил алертов на эти метрики (PromQL):

```promql
# ошибки: доля 5xx выше 1% за 5 минут
sum(rate(orders_http_requests_total{status=~"5.."}[5m]))
  / sum(rate(orders_http_requests_total[5m])) > 0.01

# латентность: p95 выше SLO (например, 300 мс)
histogram_quantile(0.95,
  sum by (le) (rate(orders_http_request_duration_seconds_bucket[5m]))) > 0.3

# outbox не выгребает: pending растёт дольше 10 минут
increase(orders_outbox_pending[10m]) > 0
```

## OpenTelemetry: обзор

Метрики отвечают «что происходит», трейсы — «почему этот запрос был
медленным». OTel — стандарт инструментовки (SDK, протокол OTLP), не
бэкенд: данные уходят в Jaeger/Tempo/свой коллектор.

**Span** — операция с именем, временем, атрибутами и контекстом; **trace** —
дерево span'ов одного запроса, связанное **trace_id**. Распространение
контекста между сервисами — заголовок `traceparent` (W3C Trace Context);
каждый сервис добавляет свои span'ы в общий трейс.

Минимальная инструментовка HTTP — `otelhttp`:

```go
handler := otelhttp.NewHandler(mux, "orders-http",
    otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
        return r.Method + " " + r.URL.Path
    }))
```

Внутри — `trace.SpanFromContext(ctx)` + `span.SetAttributes(...)`,
`span.RecordError(err)`, `span.SetStatus(codes.Error, ...)`. Трейсинг
семплируется (обычно 1–10%): трейс каждого запроса в проде — это бюджет
хранения, сравнимый с самим сервисом.

Собственный span в бизнес-логике — три строки:

```go
func (s *OrderService) CreateOrder(ctx context.Context, ...) (*domain.Order, error) {
    ctx, span := tracer.Start(ctx, "OrderService.CreateOrder")
    defer span.End()
    // ...
    span.SetAttributes(attribute.String("order.customer_id", customerID))
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
}
```

Имя span'а — стабильное (`"OrderService.CreateOrder"`, без значений): по
нему агрегируется статистика трейсинга, как по имени метрики.

Связка трёх китов:

- **лог ↔ трейс**: в логгер добавить `trace_id`/`span_id` (урок 19,
  `WithRequestID` — тот же механизм, атрибут из контекста);
- **метрика ↔ трейс**: `exemplars` (в client_golang — отдельная настройка)
  привязывают к точке метрики конкретный трейс — «вот этот запрос дал этот
  выброс»;
- на инциденте: алерт по метрике → дашборд → `trace_id` из лога → трейс →
  конкретный span и запрос.

## Go vs Python

**Python**:

```python
from prometheus_client import Counter, Histogram, generate_latest
REQUESTS = Counter("orders_http_requests_total", "...", ["method", "status"])
DURATION = Histogram("orders_http_request_duration_seconds", "...",
                     ["method", "status"], buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10))
# ...в middleware:
REQUESTS.labels(request.method, str(code)).inc()
# /metrics: generate_latest(registry)
```

`prometheus_client` — глобальные метрики-объекты на уровне модуля (паттерн
«регистрация при импорте»), `generate_latest()` отдаёт всё из registry по умолчанию. В Go та же модель, но регистр создаётся **явно** и прокидывается —
метрики тестируются изолированно (`prometheus.NewRegistry()` в тестах), без
глобального состояния и конфликтов дублей при повторной регистрации.

Трейсинг в Python — `opentelemetry-python` с автоинструментовкой
(`opentelemetry-instrument flask fastapi`): магия работает, пока не
понадобится свой span в своей логике. В Go инструментовка явная — зато
видно, где контекст и что замеряется.

## Подводные камни и анти-паттерны

1. **Кардинальность взорвалась** — `user_id`/URL в метках. Уникальные
   значения — в трейсы; в метках — ограниченные наборы.
2. **`DefaultRegisterer` в библиотеке** — дубли регистрации при импорте
   дважды, паника в среде выполнения. Свой `prometheus.NewRegistry()` и явная
   передача.
3. **`statusRecorder` без 200 по умолчанию** — `Write` без `WriteHeader` не
   засчитается, или наоборот: рекурсия `WriteHeader` → `Write` → затирание
   кода. Один код, один раз.
4. **Gauge без `defer Dec`** — при панике запрос «зависает» в in-flight
   навсегда; `defer` обязателен (и он сработает после `recover`
   middleware).
5. **Histogram с бакетами «на глаз»** — p95 всех запросов проваливается в
   последний бакет, и дашборд врёт. Бакеты — от целевых SLO.
6. **Счётчик на каждый `WithLabelValues` в цикле** — создание child-метрик
   в среде выполнения; карта кэша меток растёт, если значения не ограничены.
7. **`/metrics` на публичном порту без защиты** — утечка внутренностей
   наружу; отдельный порт/интерфейс или аутентификация.
8. **Трейсинг 100% трафика** — хранение и overhead; семплинг + отдельный
   буфер для ошибок (записывать все ошибочные трейсы — полезно).
9. **Метрика на каждый чих** — «число горутин в момент 3:14» без алерта,
   который на неё смотрит. Метрика без потребителя — шум в дашборде.

## Что делаем в итоговом проекте

1. `internal/telemetry/metrics`: `NewMiddleware(reg)` с RED-метриками
   (counter по method/status, histogram с API-бакетами, gauge in-flight);
   `/metrics` через `promhttp` на своём registry.
2. Подключаем middleware самым внешним слоем; проверяем `curl
   localhost:8080/metrics`.
3. Логгер (урок 19) получает `trace_id`/`span_id`; HTTP-обработчик
   оборачивается в `otelhttp` — трейсы связаны с логами.
4. Метрики зависимостей: латентность PostgreSQL, публикаций Kafka, размер
   outbox-очереди; алерты — на 5xx, p95 > SLO, outbox-отставание.

## Материалы для углубления

- [Prometheus: Metric Types](https://prometheus.io/docs/concepts/metric_types/) — counter/gauge/histogram/summary.
- [Prometheus: Practices — cardinality](https://prometheus.io/docs/practices/naming/) — именование и метки.
- [client_golang](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) — документация клиента.
- [RED Method](https://grafana.com/blog/2018/08/02/the-red-method-how-to-instrument-your-services/) — Tom Wilkie, как мерить сервисы.
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) — SDK, otelhttp, экспортеры.
- [W3C Trace Context](https://www.w3.org/TR/trace-context/) — стандарт `traceparent`.

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
