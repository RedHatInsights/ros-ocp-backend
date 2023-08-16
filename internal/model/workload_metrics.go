package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

type WorkloadMetrics struct {
	ID            string `gorm:"type:uuid;not null;default:uuid_generate_v4()"`
	WorkloadID    string
	Workload      Workload `gorm:"foreignKey:WorkloadID"`
	ClusterID     uint
	Cluster       Cluster `gorm:"foreignKey:ClusterID"`
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

func GetWorkloadMetricsForTimestamp(experiment_name string, interval_end time.Time) (WorkloadMetrics, error) {
	db := database.GetDB()
	var workload_metrics WorkloadMetrics
	err := db.Table("workload_metrics").Joins("JOIN workloads ON workloads.id = workload_metrics.workload_id AND workloads.experiment_name = ? AND workload_metrics.interval_end = ?", experiment_name, interval_end).Scan(&workload_metrics).Error
	return workload_metrics, err
}
