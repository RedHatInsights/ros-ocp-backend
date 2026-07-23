package api

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	http4xxTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rosocp_http_4xx_total",
		Help: "Total HTTP 4xx responses from the API",
	}, []string{"url", "http_code"})
	http5xxTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rosocp_http_5xx_total",
		Help: "Total HTTP 5xx responses from the API",
	}, []string{"url", "http_code"})
)

func recordHTTPStatusMetric(c echo.Context) {
	status := c.Response().Status
	if status < 400 {
		return
	}

	code := strconv.Itoa(status)
	path := c.Path()

	switch {
	case status >= 500:
		http5xxTotal.WithLabelValues(path, code).Inc()
	case status >= 400:
		http4xxTotal.WithLabelValues(path, code).Inc()
	}
}

func HTTPStatusMetricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		recordHTTPStatusMetric(c)
		return err
	}
}
