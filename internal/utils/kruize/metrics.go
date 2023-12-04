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
	kruizeRecommendationError = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rosocp_kruize_error_recommendations_count",
			Help: "Count of ERROR type recommendations from Kruize",
		},
		[]string{"notification_code"},
	)
)
