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
)


var (
    kruizeInvalidRecommendation = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "rosocp_kruize_invalid_recommendation_detail",
            Help: "List of INFO/ERROR type recommendations from Kruize",
        },
        []string{"notification_code", "experiment_name"},
    )
)
