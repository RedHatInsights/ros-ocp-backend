package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
)

type NamespaceRecommendationSet struct {
	ID                     string `gorm:"primaryKey;not null;autoIncrement"`
	WorkloadID             uint
	Workload               Workload `gorm:"foreignKey:WorkloadID"`
	NamespaceName          string
	CPURequestCurrent      float64
	CPUVariation           float64
	MemoryRequestCurrent   float64
	MemoryVariation        float64
	MonitoringStartTime    time.Time `gorm:"type:timestamp"`
	MonitoringEndTime      time.Time `gorm:"type:timestamp"`
	Recommendations        datatypes.JSON
	UpdatedAt              time.Time `gorm:"type:timestamp"`
	MonitoringStartTimeStr string    `gorm:"-"`
	MonitoringEndTimeStr   string    `gorm:"-"`
	UpdatedAtStr           string    `gorm:"-"`
}

type NamespaceRecommendationSetResult struct {
	ClusterAlias         string         `json:"cluster_alias"`
	ClusterUUID          string         `json:"cluster_uuid"`
	ID                   string         `json:"id"`
	LastReported         string         `json:"last_reported"`
	Project              string         `json:"namespace_name"`
	CPURequestCurrent    float64        `json:"cpu_request_current"`
	CPUvariation         float64        `json:"cpu_variation"`
	MemoryRequestCurrent float64        `json:"memory_request_current"`
	MemoryVariation      float64        `json:"memory_variation"`
	Recommendations      datatypes.JSON `json:"-"`
	RecommendationsJSON  map[string]any `gorm:"-" json:"recommendations"`
	SourceID             string         `json:"source_id"`
	Workload             string         `json:"workload"`
	WorkloadType         string         `json:"workload_type"`
}

func (r *NamespaceRecommendationSet) AfterFind(tx *gorm.DB) error {
	r.MonitoringEndTimeStr = r.MonitoringEndTime.Format(time.RFC3339)
	return nil
}

func (r *NamespaceRecommendationSet) GetNamespaceRecommendationSets(orgID string, orderQuery string, format string, limit int, offset int, queryParams map[string]interface{}, user_permissions map[string][]string) ([]NamespaceRecommendationSetResult, int, error) {
	var recommendationSets []NamespaceRecommendationSetResult
	query := getNamespaceRecommendationQuery(orgID)

	add_rbac_filter(query, user_permissions)

	for key, values := range queryParams {
		switch v := values.(type) {
		case []string:
			args := make([]any, len(v))
			for i, s := range v {
				args[i] = s
			}
			query = query.Where(key, args...)
		default:
			query = query.Where(key, v)
		}
	}

	var count int64 = 0
	query.Count(&count)
	query.Order(orderQuery)

	if format == "csv" {
		limit = config.GetConfig().RecordLimitCSV
	}
	err := query.Offset(offset).Limit(limit).Scan(&recommendationSets).Error

	return recommendationSets, int(count), err

}

func (r *NamespaceRecommendationSet) CreateNamespaceRecommendationSet(tx *gorm.DB) error {
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workload_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"monitoring_start_time", "monitoring_end_time", "recommendations", "updated_at"}),
	}).Create(r)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}
