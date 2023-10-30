package utils

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	invalidDataPoints = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_datapoints_total",
		Help: "The total number of invalid datapoints(rows) found in CSVs recevied",
	})
)
