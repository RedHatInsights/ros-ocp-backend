package model

import (
	"time"

	"gorm.io/datatypes"
)

type RecommendationSet struct {
	WorkloadID      uint
	Workload        Workload
	Recommendations datatypes.JSON
	CreatedAt       time.Time
}
