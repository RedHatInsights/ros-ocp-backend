package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	invalidCSV = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_csv_total",
		Help: "The total number of invalid container csv send by cost-mgmt",
	})
	recommendationRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_recommendation_request_total",
		Help: "The total number of container recommendations requested from Kruize",
	})
	namespaceRecommendationRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_namespace_recommendation_request_total",
		Help: "The total number of namespace recommendations requested from Kruize",
	})
	recommendationSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_recommendation_success_total",
		Help: "The total number of container recommendations saved by ROSOCP",
	})
	namespaceRecommendationSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_namespace_recommendation_success_total",
		Help: "The total number of namespace recommendations saved by ROSOCP",
	})
	invalidNamespaceCSV = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_namespace_csv_total",
		Help: "The total number of invalid namespace csvs sent by cost-mgmt",
	})
	csvFetchError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_csv_fetch_error_total",
		Help: "The total number of errors encountered while fetching CSV from URL",
	})
)
