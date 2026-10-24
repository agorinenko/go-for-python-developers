// Package metrics — RED-метрики HTTP API сервиса заказов
// (CAPSTONE.md, неделя 10: internal/telemetry/metrics + /metrics).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// Полные имена метрик — с namespace "orders".
const (
	// RequestsTotalName — счётчик запросов (метки method, status).
	RequestsTotalName = "orders_http_requests_total"
	// RequestDurationSecondsName — гистограмма длительности (метки method, status).
	RequestDurationSecondsName = "orders_http_request_duration_seconds"
	// InFlightRequestsName — gauge числа выполняющихся запросов.
	InFlightRequestsName = "orders_http_in_flight_requests"
)

// durationBuckets — бакеты гистограммы длительности запросов API:
// от 5 мс до 10 с.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// NewMiddleware регистрирует метрики в reg и возвращает middleware.
// Если reg == nil, используется prometheus.DefaultRegisterer.
func NewMiddleware(reg prometheus.Registerer) func(http.Handler) http.Handler {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "orders",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests by method and status.",
	}, []string{"method", "status"})
	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "orders",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds by method and status.",
		Buckets:   durationBuckets,
	}, []string{"method", "status"})
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "orders",
		Subsystem: "http",
		Name:      "in_flight_requests",
		Help:      "Number of HTTP requests currently being handled.",
	})
	reg.MustRegister(requestsTotal, requestDuration, inFlight)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: реализуй (лекция, раздел «RED-метрики»):
			//  1. inFlight.Inc() в начале, inFlight.Dec() — через defer;
			//  2. start := time.Now(); через defer:
			//     requestDuration.WithLabelValues(r.Method, status).
			//     Observe(time.Since(start).Seconds());
			//  3. оберни w в statusRecorder (тип ниже): он перехватывает
			//     код ответа, по умолчанию — 200;
			//  4. после next.ServeHTTP(rec, r):
			//     requestsTotal.WithLabelValues(r.Method,
			//     strconv.Itoa(rec.code)).Inc().
			// Для time и strconv добавь импорты.
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder — обёртка http.ResponseWriter, запоминающая код ответа.
// Код выставляется при первом WriteHeader, либо 200 при первом Write.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

// WriteHeader запоминает код (если ещё не записан) и делегирует вызов.
//
// TODO: реализуй:
// если r.code == 0 — запомнить code; затем r.ResponseWriter.WriteHeader(code).
func (r *statusRecorder) WriteHeader(code int) {
	// TODO: реализуй. Заглушка не запоминает код — счётчики по статусам
	// не сработают.
	r.ResponseWriter.WriteHeader(code)
}

// Write подразумевает 200, если код ещё не установлен, и делегирует вызов.
//
// TODO: реализуй:
// если r.code == 0 — установить http.StatusOK; затем вернуть r.ResponseWriter.Write(b).
func (r *statusRecorder) Write(b []byte) (int, error) {
	// TODO: реализуй. Заглушка не фиксирует 200 для «тихого» Write.
	return r.ResponseWriter.Write(b)
}
