package model

import (
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/redhatinsights/ros-ocp-backend/internal/api/listoptions"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/rbac"
)

type NamespaceRecommendationSet struct {
	ID                   string `gorm:"primaryKey;not null;autoIncrement"`
	OrgID                string `gorm:"type:text;not null"`
	WorkloadID           uint
	Workload             Workload `gorm:"foreignKey:WorkloadID"`
	NamespaceName        string
	CPURequestCurrent    float64
	MemoryRequestCurrent float64

	// Variation fields: percent of current CPU/memory request (aligned with API response).
	CPUVariationShortCostPct            float64 `gorm:"column:cpu_variation_short_cost_pct"`
	CPUVariationShortPerformancePct     float64 `gorm:"column:cpu_variation_short_performance_pct"`
	CPUVariationMediumCostPct           float64 `gorm:"column:cpu_variation_medium_cost_pct"`
	CPUVariationMediumPerformancePct    float64 `gorm:"column:cpu_variation_medium_performance_pct"`
	CPUVariationLongCostPct             float64 `gorm:"column:cpu_variation_long_cost_pct"`
	CPUVariationLongPerformancePct      float64 `gorm:"column:cpu_variation_long_performance_pct"`
	MemoryVariationShortCostPct         float64 `gorm:"column:memory_variation_short_cost_pct"`
	MemoryVariationShortPerformancePct  float64 `gorm:"column:memory_variation_short_performance_pct"`
	MemoryVariationMediumCostPct        float64 `gorm:"column:memory_variation_medium_cost_pct"`
	MemoryVariationMediumPerformancePct float64 `gorm:"column:memory_variation_medium_performance_pct"`
	MemoryVariationLongCostPct          float64 `gorm:"column:memory_variation_long_cost_pct"`
	MemoryVariationLongPerformancePct   float64 `gorm:"column:memory_variation_long_performance_pct"`

	MonitoringStartTime    time.Time `gorm:"type:timestamp"`
	MonitoringEndTime      time.Time `gorm:"type:timestamp"`
	Recommendations        datatypes.JSON
	CreatedAt              time.Time `gorm:"type:timestamp with time zone;not null;default:now();<-:create"`
	UpdatedAt              time.Time `gorm:"type:timestamp"`
	MonitoringStartTimeStr string    `gorm:"-"`
	MonitoringEndTimeStr   string    `gorm:"-"`
	UpdatedAtStr           string    `gorm:"-"`
}

type NamespaceRecommendationSetResult struct {
	ClusterAlias        string         `json:"cluster_alias"`
	ClusterUUID         string         `json:"cluster_uuid"`
	ID                  string         `json:"id"`
	LastReported        string         `json:"last_reported"`
	Project             string         `json:"project"`
	Recommendations     datatypes.JSON `json:"-"`
	RecommendationsJSON map[string]any `gorm:"-" json:"recommendations"`
	SourceID            string         `json:"source_id"`
}

func (r *NamespaceRecommendationSet) AfterFind(tx *gorm.DB) error {
	r.MonitoringEndTimeStr = r.MonitoringEndTime.Format(time.RFC3339)
	return nil
}

func (r *NamespaceRecommendationSet) GetNamespaceRecommendationSets(orgID string, opts listoptions.ListOptions, queryParams map[string]interface{}, user_permissions map[string][]string) ([]NamespaceRecommendationSetResult, int, error) {
	var recommendationSets []NamespaceRecommendationSetResult
	var count int64 = 0
	query := getNamespaceRecommendationQuery(orgID)

	if err := rbac.AddRBACFilter(
		query,
		user_permissions,
		rbac.ResourceProject,
	); err != nil {
		return recommendationSets, int(count), err
	}

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

	query.Count(&count)
	// Secondary sort by primary key for stable ordering when the primary sort column ties.
	query = query.Order(opts.OrderBy + " " + opts.OrderHow).Order("namespace_recommendation_sets.id ASC")

	limit := opts.Limit
	if opts.Format == "csv" {
		limit = config.GetConfig().RecordLimitCSV
	}
	err := query.Offset(opts.Offset).Limit(limit).Scan(&recommendationSets).Error

	return recommendationSets, int(count), err

}

func (r *NamespaceRecommendationSet) GetNamespaceRecommendationSetByID(orgID string, recommendationID string, user_permissions map[string][]string) (NamespaceRecommendationSetResult, error) {
	var nsRecommendationSet NamespaceRecommendationSetResult

	query := getNamespaceRecommendationQuery(orgID)
	query.Where("namespace_recommendation_sets.id = ?", recommendationID)

	if err := rbac.AddRBACFilter(
		query,
		user_permissions,
		rbac.ResourceProject,
	); err != nil {
		return nsRecommendationSet, err
	}

	err := query.First(&nsRecommendationSet).Error
	return nsRecommendationSet, err
}

func (r *NamespaceRecommendationSet) CreateNamespaceRecommendationSet(tx *gorm.DB) error {
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workload_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"monitoring_start_time",
			"monitoring_end_time",
			"recommendations",
			"updated_at",
			"cpu_request_current",
			"memory_request_current",
			"cpu_variation_short_cost_pct",
			"cpu_variation_short_performance_pct",
			"cpu_variation_medium_cost_pct",
			"cpu_variation_medium_performance_pct",
			"cpu_variation_long_cost_pct",
			"cpu_variation_long_performance_pct",
			"memory_variation_short_cost_pct",
			"memory_variation_short_performance_pct",
			"memory_variation_medium_cost_pct",
			"memory_variation_medium_performance_pct",
			"memory_variation_long_cost_pct",
			"memory_variation_long_performance_pct",
		}),
	}).Create(r)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}

func GetFirstNamespaceRecommendationSetsByWorkloadID(workload_id uint) (NamespaceRecommendationSet, error) {
	namespaceRecommendationSets := NamespaceRecommendationSet{}
	db := database.GetDB()
	query := db.Where("workload_id = ?", workload_id).First(&namespaceRecommendationSets)
	if query.Error != nil && errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return namespaceRecommendationSets, nil
	}
	return namespaceRecommendationSets, query.Error
}
