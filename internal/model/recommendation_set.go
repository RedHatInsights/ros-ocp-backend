package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
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

func (r *RecommendationSet) CreateRecommendationSet() error {
	db := database.GetDB()
	result := db.Create(r)

	if result.Error != nil {
		return result.Error
	}

	return nil
}
