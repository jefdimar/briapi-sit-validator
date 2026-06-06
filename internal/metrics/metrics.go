package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HttpRequestsTotal tracks the total number of HTTP requests.
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HttpRequestDurationSeconds tracks HTTP request latencies.
	HttpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// ValidationSheetsTotal tracks validation results at the sheet level.
	ValidationSheetsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "validation_sheets_total",
			Help: "Total number of processed sheets by validation status.",
		},
		[]string{"status"},
	)

	// ValidationTestCasesTotal tracks validation results at the test case level.
	ValidationTestCasesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "validation_test_cases_total",
			Help: "Total number of validated test cases by validation status.",
		},
		[]string{"status"},
	)

	// UniqueHeaderViolationsTotal tracks unique header violations.
	UniqueHeaderViolationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "validation_unique_header_violations_total",
			Help: "Total number of unique header violations detected.",
		},
		[]string{"header"},
	)
)
