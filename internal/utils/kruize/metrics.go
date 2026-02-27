package kruize

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	kruizeAPIException = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rosocp_kruize_api_exception_total",
		Help: "The total number of exception got while calling kruize API",
	},
		[]string{"path"},
	)
	invalidRecommendation = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_recommendation_total",
		Help: "The total number of invalid container recommendations send by Kruize",
	})
	invalidNamespaceRecommendation = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_namespace_recommendation_total",
		Help: "The total number of invalid namespace recommendations send by Kruize",
	})
	createExperimentRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_create_experiment_request_total",
		Help: "The total number of container experiment creation requests sent to Kruize",
	})
	updateResultRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_update_result_request_total",
		Help: "The total number of container update result requests sent to Kruize",
	})
	createNamespaceExperimentRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_create_namespace_experiment_request_total",
		Help: "The total number of namespace experiment creation requests sent to Kruize",
	})
	updateNamespaceResultRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_update_namespace_result_request_total",
		Help: "The total number of namespace update result requests sent to Kruize",
	})
)
