package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"practice/metrics"
)

// dtoAdapter адаптирует собранную *dto.Metric к интерфейсу prometheus.Metric,
// чтобы testutil.ToFloat64 могла прочитать counter/gauge по меткам.
type dtoAdapter struct {
	desc *prometheus.Desc
	m    *dto.Metric
}

func (d *dtoAdapter) Desc() *prometheus.Desc { return d.desc }

func (d *dtoAdapter) Write(out *dto.Metric) error {
	out.Label = d.m.GetLabel()
	out.Counter = d.m.GetCounter()
	out.Gauge = d.m.GetGauge()
	out.Histogram = d.m.GetHistogram()
	out.Summary = d.m.GetSummary()
	out.Untyped = d.m.GetUntyped()
	return nil
}

// singleCollector — обёртка, отдающая ровно одну метрику: testutil.ToFloat64
// принимает Collector и падает, если метрик больше одной.
type singleCollector struct {
	m prometheus.Metric
}

func (c singleCollector) Describe(chan<- *prometheus.Desc) {}

func (c singleCollector) Collect(ch chan<- prometheus.Metric) { ch <- c.m }

// findMetric ищет метрику с заданными метками в собранных семействах.
func findMetric(t *testing.T, reg *prometheus.Registry, name string, wantLabels map[string]string) *dto.Metric {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			got := map[string]string{}
			for _, lp := range m.GetLabel() {
				got[lp.GetName()] = lp.GetValue()
			}
			match := len(got) == len(wantLabels)
			if match {
				for k, v := range wantLabels {
					if got[k] != v {
						match = false
						break
					}
				}
			}
			if match {
				return m
			}
		}
	}
	t.Fatalf("метрика %s с метками %v не найдена", name, wantLabels)
	return nil
}

// metricValue — значение counter/gauge по меткам через testutil.ToFloat64.
func metricValue(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	m := findMetric(t, reg, name, labels)
	return testutil.ToFloat64(singleCollector{
		&dtoAdapter{desc: prometheus.NewDesc(name, "", nil, nil), m: m},
	})
}

// histogramCount — sample count гистограммы по меткам.
// testutil.ToFloat64 для гистограммы не подходит (она принимает только
// counter/gauge/untyped), поэтому count читается из dto.
func histogramCount(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) uint64 {
	t.Helper()
	return findMetric(t, reg, name, labels).GetHistogram().GetSampleCount()
}

// newServer собирает middleware поверх обработчика: /fail отвечает 500,
// остальное — 200 с телом "ok".
func newServer(t *testing.T) (*prometheus.Registry, http.Handler) {
	t.Helper()
	reg := prometheus.NewRegistry()
	mw := metrics.NewMiddleware(reg)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fail":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte("ok")) // Write без WriteHeader — код по умолчанию 200
		}
	})
	return reg, mw(next)
}

func doRequest(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	h.ServeHTTP(rec, req)
	return rec
}

func TestMiddlewarePassesResponseThrough(t *testing.T) {
	_, h := newServer(t)

	rec := doRequest(t, h, http.MethodGet, "/ok")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q — middleware не должен менять ответ", rec.Body.String(), "ok")
	}
}

func TestMiddlewareCounters(t *testing.T) {
	reg, h := newServer(t)

	doRequest(t, h, http.MethodGet, "/ok")
	doRequest(t, h, http.MethodGet, "/ok")
	doRequest(t, h, http.MethodPost, "/fail")

	got := metricValue(t, reg, metrics.RequestsTotalName, map[string]string{"method": "GET", "status": "200"})
	if got != 2 {
		t.Fatalf("requests_total{method=GET,status=200} = %v, want 2", got)
	}
	got = metricValue(t, reg, metrics.RequestsTotalName, map[string]string{"method": "POST", "status": "500"})
	if got != 1 {
		t.Fatalf("requests_total{method=POST,status=500} = %v, want 1", got)
	}

	count := histogramCount(t, reg, metrics.RequestDurationSecondsName, map[string]string{"method": "GET", "status": "200"})
	if count != 2 {
		t.Fatalf("request_duration_seconds{GET,200} count = %d, want 2", count)
	}
	count = histogramCount(t, reg, metrics.RequestDurationSecondsName, map[string]string{"method": "POST", "status": "500"})
	if count != 1 {
		t.Fatalf("request_duration_seconds{POST,500} count = %d, want 1", count)
	}
}

func TestMiddlewareGaugeReturnsToZero(t *testing.T) {
	reg, h := newServer(t)

	doRequest(t, h, http.MethodGet, "/ok")
	doRequest(t, h, http.MethodGet, "/ok")

	got := metricValue(t, reg, metrics.InFlightRequestsName, map[string]string{})
	if got != 0 {
		t.Fatalf("in_flight_requests = %v, want 0 — gauge обязан вернуться к нулю после завершения запросов", got)
	}
}
