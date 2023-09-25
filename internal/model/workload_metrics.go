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
	IntervalStart time.Time `gorm:"type:timestamp"`
	IntervalEnd   time.Time `gorm:"type:timestamp"`
	UsageMetrics  datatypes.JSON
}

func (w *WorkloadMetrics) CreateWorkloadMetrics() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "workload_id"}, {Name: "container_name"}, {Name: "interval_start"}, {Name: "interval_end"}},
		DoNothing: true,
	}).Create(w)

	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "no partition") {
			partitionMissing.With(prometheus.Labels{"resource_name": "workload_metrics"}).Inc()
			dbError.Inc()
			return fmt.Errorf("parition not found for resource %s with org_id %s and end_time %s", "workload_metrics", w.OrgId, w.IntervalEnd.String())
		}
		dbError.Inc()
		return result.Error
	}

	return nil
}
