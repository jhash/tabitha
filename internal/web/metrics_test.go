package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsMiddlewareCountsRequestsByStatus(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newRequestMetrics(reg)

	handler := m.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	rec := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `tabitha_http_requests_total`) {
		t.Errorf("expected a tabitha_http_requests_total metric, got: %s", body)
	}
	if !strings.Contains(body, `status="200"`) {
		t.Errorf("expected the request's status code recorded, got: %s", body)
	}
}

func TestMetricsMiddlewareRecordsRequestDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newRequestMetrics(reg)

	handler := m.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	rec := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !strings.Contains(rec.Body.String(), "tabitha_http_request_duration_seconds") {
		t.Errorf("expected a request duration histogram, got: %s", rec.Body.String())
	}
}
