package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// requestMetrics tracks per-request counters/histograms for the <100ms
// public-render SLO (see docs/monitoring.md): a request count by
// method/status, and a duration histogram to check against that target.
type requestMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func newRequestMetrics(reg prometheus.Registerer) *requestMetrics {
	m := &requestMetrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tabitha_http_requests_total",
			Help: "Total HTTP requests, by method and response status.",
		}, []string{"method", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tabitha_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, by method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),
	}
	reg.MustRegister(m.requestsTotal, m.requestDuration)
	return m
}

// statusRecorder captures the status code a handler writes, so it can be
// reported after the fact — http.ResponseWriter itself doesn't expose it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (m *requestMetrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		m.requestsTotal.WithLabelValues(r.Method, strconv.Itoa(rec.status)).Inc()
		m.requestDuration.WithLabelValues(r.Method).Observe(time.Since(start).Seconds())
	})
}
