package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

type WorkloadMetrics struct {
	ID            uint `gorm:"primaryKey;not null;autoIncrement"`
	WorkloadID    uint
	Workload      Workload `gorm:"foreignKey:WorkloadID"`
	ContainerName string
	IntervalStart time.Time `gorm:"type:timestamp"`
	IntervalEnd   time.Time `gorm:"type:timestamp"`
	UsageMetrics  datatypes.JSON
}

func (w *WorkloadMetrics) CreateWorkloadMetrics() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workload_id"}, {Name: "container_name"}, {Name: "interval_start"}, {Name: "interval_end"}},
		DoUpdates: clause.AssignmentColumns([]string{"usage_metrics"}),
	}).Create(w)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}
