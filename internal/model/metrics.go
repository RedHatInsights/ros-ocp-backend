package model

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dbError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_db_error_total",
		Help: "The total number of DB error",
	})
	partitionMissing = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rosocp_partition__missing_error_total",
		Help: "The total number of DB error due to table partition does not exist",
	},
		[]string{"resource_name"},
	)
	rhAccountCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_rh_account_created_total",
		Help: "The total number of rh account created",
	})
)
