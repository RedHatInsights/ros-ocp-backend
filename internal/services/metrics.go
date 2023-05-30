package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	invalidRecommendation = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_recommendation_total",
		Help: "The total number of invalid recommendation send by Kruize",
	})
	invalidCSV = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_csv_total",
		Help: "The total number of invalid csv send by cost-mgmt",
	})
)
