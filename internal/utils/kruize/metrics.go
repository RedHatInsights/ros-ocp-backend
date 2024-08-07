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
		Help: "The total number of invalid recommendation send by Kruize",
	})
	createExperimentRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_create_experiment_request_total",
		Help: "The total number of experiment creation requests sent to Kruize",
	})
	updateResultRequest = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kruize_update_result_request_total",
		Help: "The total number of requests sent to Kruize for UpdateResult",
	})
)
