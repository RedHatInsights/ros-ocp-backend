package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

type WorkloadMetrics struct {
	ID            uint   `gorm:"primaryKey;not null;autoIncrement"`
	OrgId         string `gorm:"type:text;not null"`
	WorkloadID    uint
	Workload      Workload `gorm:"foreignKey:WorkloadID"`
	ContainerName string
	NamespaceName string
	MetricType    string    `gorm:"type:metrictype;default:'container'"`
	IntervalStart time.Time `gorm:"type:timestamp"`
	IntervalEnd   time.Time `gorm:"type:timestamp"`
	UsageMetrics  datatypes.JSON
}

func BatchInsertWorkloadMetrics(data []WorkloadMetrics, org_id string) error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "workload_id"}, {Name: "container_name"}, {Name: "interval_start"}, {Name: "interval_end"}},
		DoNothing: true,
	}).Create(data)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "no partition") {
			partitionMissing.With(prometheus.Labels{"resource_name": "workload_metrics"}).Inc()
			dbError.Inc()
			return fmt.Errorf("partition not found for resource %s with org_id %s", "workload_metrics", org_id)
		}
		dbError.Inc()
		return result.Error
	}
	return nil
}
