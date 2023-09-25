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

type HistoricalRecommendationSet struct {
	ID                  uint   `gorm:"primaryKey;not null;autoIncrement"`
	OrgId               string `gorm:"type:text;not null"`
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
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "workload_id"}, {Name: "container_name"}, {Name: "monitoring_end_time"}},
		DoUpdates: clause.AssignmentColumns([]string{"monitoring_start_time", "monitoring_end_time", "recommendations", "updated_at"}),
	}).Create(r)

	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "no partition") {
			partitionMissing.With(prometheus.Labels{"resource_name": "historical_recommendation_set"}).Inc()
			dbError.Inc()
			return fmt.Errorf("partition not found for resource %s with org_id %s and end_time %s", "historical_recommendation_set", r.OrgId, r.MonitoringEndTime.String())
		}
		dbError.Inc()
		return result.Error
	}

	return nil
}
