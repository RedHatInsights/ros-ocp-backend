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
	rhAccountCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_rh_account_created_total",
		Help: "The total number of rh account created",
	})
)
