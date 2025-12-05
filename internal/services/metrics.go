package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	invalidCSV = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_csv_total",
		Help: "The total number of invalid csv send by cost-mgmt",
	})
	recommendationRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_recommendation_request_total",
		Help: "The total number of recommendations requested from Kruize",
	})
	recommendationSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_recommendation_success_total",
		Help: "The total number of recommendations saved by ROSOCP",
	})
	//nolint:unused
	invalidNamespaceCSV = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_namespace_csv_total",
		Help: "The total number of invalid namespace csvs sent by cost-mgmt",
	})
)
