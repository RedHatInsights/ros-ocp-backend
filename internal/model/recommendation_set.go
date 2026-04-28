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

type RecommendationSet struct {
	ID            string `gorm:"primaryKey;not null;autoIncrement"`
	WorkloadID    uint
	Workload      Workload `gorm:"foreignKey:WorkloadID"`
	ContainerName string

	CPURequestCurrent    *float64 `gorm:"column:cpu_request_current;type:numeric(10,4)"`
	MemoryRequestCurrent *float64 `gorm:"column:memory_request_current;type:numeric(20,4)"`

	// Variation fields: percent of current CPU/memory request (aligned with API response).
	CPUVariationShortCostPct            *float64 `gorm:"column:cpu_variation_short_cost_pct;type:numeric(10,4)"`
	CPUVariationShortPerformancePct     *float64 `gorm:"column:cpu_variation_short_performance_pct;type:numeric(10,4)"`
	CPUVariationMediumCostPct           *float64 `gorm:"column:cpu_variation_medium_cost_pct;type:numeric(10,4)"`
	CPUVariationMediumPerformancePct    *float64 `gorm:"column:cpu_variation_medium_performance_pct;type:numeric(10,4)"`
	CPUVariationLongCostPct             *float64 `gorm:"column:cpu_variation_long_cost_pct;type:numeric(10,4)"`
	CPUVariationLongPerformancePct      *float64 `gorm:"column:cpu_variation_long_performance_pct;type:numeric(10,4)"`
	MemoryVariationShortCostPct         *float64 `gorm:"column:memory_variation_short_cost_pct;type:numeric(10,4)"`
	MemoryVariationShortPerformancePct  *float64 `gorm:"column:memory_variation_short_performance_pct;type:numeric(10,4)"`
	MemoryVariationMediumCostPct        *float64 `gorm:"column:memory_variation_medium_cost_pct;type:numeric(10,4)"`
	MemoryVariationMediumPerformancePct *float64 `gorm:"column:memory_variation_medium_performance_pct;type:numeric(10,4)"`
	MemoryVariationLongCostPct          *float64 `gorm:"column:memory_variation_long_cost_pct;type:numeric(10,4)"`
	MemoryVariationLongPerformancePct   *float64 `gorm:"column:memory_variation_long_performance_pct;type:numeric(10,4)"`

	MonitoringStartTime    time.Time `gorm:"type:timestamp"`
	MonitoringEndTime      time.Time `gorm:"type:timestamp"`
	Recommendations        datatypes.JSON
	UpdatedAt              time.Time `gorm:"type:timestamp"`
	MonitoringStartTimeStr string    `gorm:"-"`
	MonitoringEndTimeStr   string    `gorm:"-"`
	UpdatedAtStr           string    `gorm:"-"`
}

type RecommendationSetResult struct {
	/*
		Intended to be an API-ready struct.
		Updated recommendation data is saved to RecommendationsJSON before the API response is sent.
	*/
	ClusterAlias        string                 `json:"cluster_alias"`
	ClusterUUID         string                 `json:"cluster_uuid"`
	Container           string                 `json:"container"`
	ID                  string                 `json:"id"`
	LastReported        string                 `json:"last_reported"`
	Project             string                 `json:"project"`
	Recommendations     datatypes.JSON         `json:"-"`
	RecommendationsJSON map[string]interface{} `gorm:"-" json:"recommendations"`
	SourceID            string                 `json:"source_id"`
	Workload            string                 `json:"workload"`
	WorkloadType        string                 `json:"workload_type"`
	// Embedded stored variation percentages (scanned from SELECT, excluded from JSON output).
	StoredVariationPcts `gorm:"embedded"`
}

func (r *RecommendationSet) AfterFind(tx *gorm.DB) error {
	r.MonitoringEndTimeStr = r.MonitoringEndTime.Format(time.RFC3339)
	return nil
}

func GetFirstRecommendationSetsByWorkloadID(workload_id uint) (RecommendationSet, error) {
	recommendationSets := RecommendationSet{}
	db := database.GetDB()
	query := db.Where("workload_id = ?", workload_id).First(&recommendationSets)
	if query.Error != nil && errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return recommendationSets, nil
	}
	return recommendationSets, query.Error
}

func (r *RecommendationSet) GetRecommendationSets(orgID string, opts listoptions.ListOptions, queryParams map[string]interface{}, user_permissions map[string][]string) ([]RecommendationSetResult, int, error) {
	var recommendationSets []RecommendationSetResult
	var count int64 = 0
	query := getRecommendationQuery(orgID)

	if err := rbac.AddRBACFilter(
		query,
		user_permissions,
		rbac.ResourceContainer,
	); err != nil {
		return recommendationSets, int(count), err
	}

	for key, values := range queryParams {
		switch v := values.(type) {
		case []string:
			// Convert []string to []interface{} for unpacking multiple values
			args := make([]interface{}, len(v))
			for i, s := range v {
				args[i] = s
			}
			query = query.Where(key, args...)
		default:
			query = query.Where(key, v)
		}
	}

	query.Count(&count)
	// OrderBy/OrderHow come from ListAPIOptions (allowlisted); secondary sort for stable ordering.
	query = query.Order(listoptions.SQLOrderByFragment(opts.OrderBy, opts.OrderHow)).Order("recommendation_sets.id ASC")

	limit := opts.Limit
	if opts.Format == "csv" {
		/*
		 each db record has short, medium, long term recommendations
		 each such term recommendation has two types, cost and performance
		 total number of CSV rows would be RecordLimitCSV * 3 * 2
		*/
		limit = config.GetConfig().RecordLimitCSV
	}
	err := query.Offset(opts.Offset).Limit(limit).Scan(&recommendationSets).Error

	return recommendationSets, int(count), err
}

func (r *RecommendationSet) GetRecommendationSetByID(orgID string, recommendationID string, user_permissions map[string][]string) (RecommendationSetResult, error) {
	var recommendationSet RecommendationSetResult

	query := getRecommendationQuery(orgID)
	query.Where("recommendation_sets.id = ?", recommendationID)

	if err := rbac.AddRBACFilter(
		query,
		user_permissions,
		rbac.ResourceContainer,
	); err != nil {
		return recommendationSet, err
	}

	err := query.First(&recommendationSet).Error
	return recommendationSet, err
}

func (r *RecommendationSet) CreateRecommendationSet(tx *gorm.DB) error {
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workload_id"}, {Name: "container_name"}},
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
