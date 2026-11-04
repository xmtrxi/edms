// Package observability exposes Prometheus metrics and OpenTelemetry traces.
package observability

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_http_requests_total",
		Help: "HTTP requests by method, route and status.",
	}, []string{"method", "route", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "edms_http_request_duration_seconds",
		Help:    "HTTP request latency by method and route.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"method", "route"})
	httpInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edms_http_requests_in_flight",
		Help: "HTTP requests being served.",
	})

	// Uploads counts stored files by outcome: stored, duplicate, blocked or failed.
	Uploads = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_uploads_total",
		Help: "Uploaded files by outcome.",
	}, []string{"outcome"})
	// MalwareScans counts scans by result: clean, infected or error.
	MalwareScans = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_malware_scans_total",
		Help: "Malware scans by result.",
	}, []string{"result"})
	// Emails counts delivery attempts and provider callbacks by status.
	Emails = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_emails_total",
		Help: "Email delivery status changes.",
	}, []string{"status"})
	// PDFJobs counts finished PDF tool runs.
	PDFJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_pdf_jobs_total",
		Help: "PDF tool jobs by operation and outcome.",
	}, []string{"operation", "status"})
	// Logins counts sign-in attempts by result.
	Logins = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_logins_total",
		Help: "Sign-in attempts by method and result.",
	}, []string{"method", "result"})
	// AccessChecks times permission decisions, which every document request
	// makes, by kind and whether access was allowed.
	AccessChecks = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "edms_access_check_duration_seconds",
		Help:    "Permission check latency by kind and decision.",
		Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25},
	}, []string{"kind", "allowed"})
	// Notifications counts in-app notifications created by kind.
	Notifications = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edms_notifications_total",
		Help: "Notifications created by kind.",
	}, []string{"kind"})
)

// Middleware records request counts and latency by route template, so
// /documents/:id is one series however many documents there are.
func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			httpInFlight.Inc()
			defer httpInFlight.Dec()
			err := next(c)

			route := c.Path()
			if route == "" {
				route = "unmatched"
			}
			status := statusOf(c, err)
			method := c.Request().Method
			httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
			httpDuration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

// statusOf reads the response status, including errors not yet written.
func statusOf(c *echo.Context, err error) int {
	if err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			return httpErr.Code
		}
		return http.StatusInternalServerError
	}
	if resp, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && resp.Status != 0 {
		return resp.Status
	}
	return http.StatusOK
}

// Handler serves the metrics in Prometheus text format.
func Handler() http.Handler {
	return promhttp.Handler()
}

// ObserveAccess times a permission check.
func ObserveAccess(kind string, start time.Time, allowed bool) {
	AccessChecks.WithLabelValues(kind, strconv.FormatBool(allowed)).Observe(time.Since(start).Seconds())
}
