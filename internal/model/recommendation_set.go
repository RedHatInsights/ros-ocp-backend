package model

import (
	"time"

	"gorm.io/datatypes"
)

type RecommendationSet struct {
	WorkloadID          uint
	Workload            Workload
	MonitoringStartTime time.Time
	MonitoringEndTime   time.Time
	Recommendations     datatypes.JSON
	CreatedAt           time.Time
}
