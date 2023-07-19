package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

type HistoricalRecommendationSet struct {
	ID                  string `gorm:"primaryKey;not null;autoIncrement"`
	WorkloadID          uint
	Workload            Workload `gorm:"foreignKey:WorkloadID"`
	ContainerName       string
	MonitoringStartTime time.Time `gorm:"type:timestamp"`
	MonitoringEndTime   time.Time `gorm:"type:timestamp"`
	Recommendations     datatypes.JSON
	UpdatedAt           time.Time `gorm:"type:timestamp"`
}

func (r *HistoricalRecommendationSet) CreateHistoricalRecommendationSet() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workload_id"}, {Name: "container_name"}, {Name: "monitoring_end_time"}},
		DoUpdates: clause.AssignmentColumns([]string{"monitoring_start_time", "monitoring_end_time", "recommendations", "updated_at"}),
	}).Create(r)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}
