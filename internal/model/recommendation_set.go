package model

import (
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

type RecommendationSet struct {
	ID                     string `gorm:"primaryKey;not null;autoIncrement"`
	WorkloadID             uint
	Workload               Workload `gorm:"foreignKey:WorkloadID"`
	ContainerName          string
	MonitoringStartTime    time.Time `gorm:"type:timestamp"`
	MonitoringEndTime      time.Time `gorm:"type:timestamp"`
	Recommendations        datatypes.JSON
	UpdatedAt              time.Time `gorm:"type:timestamp"`
	MonitoringStartTimeStr string    `gorm:"-"`
	MonitoringEndTimeStr   string    `gorm:"-"`
	UpdatedAtStr           string    `gorm:"-"`
}

func (r *RecommendationSet) AfterFind(tx *gorm.DB) error {
	r.MonitoringStartTimeStr = r.MonitoringStartTime.Format(time.RFC3339)
	r.MonitoringEndTimeStr = r.MonitoringEndTime.Format(time.RFC3339)
	r.UpdatedAtStr = r.UpdatedAt.Format(time.RFC3339)
	return nil
}

func (r *RecommendationSet) GetRecommendationSets(orgID string, orderQuery string, limit int, offset int, queryParams map[string]interface{}) ([]RecommendationSet, int, error) {

	var recommendationSets []RecommendationSet
	db := database.GetDB()

	query := db.Table("recommendation_sets").Joins(`
		JOIN (
			SELECT workload_id, MAX(monitoring_end_time) AS latest_monitoring_end_time 
			FROM recommendation_sets GROUP BY workload_id, container_name
		) latest_rs ON recommendation_sets.workload_id = latest_rs.workload_id 
				AND recommendation_sets.monitoring_end_time = latest_rs.latest_monitoring_end_time
			JOIN workloads ON recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
			JOIN rh_accounts ON clusters.tenant_id = rh_accounts.id
		`).Model(r).Preload("Workload.Cluster.RHAccount").Where("rh_accounts.org_id = ?", orgID)

	for key, value := range queryParams {
		if strings.Contains(key, "clusters") {
			clusterQuery := "clusters.cluster_alias LIKE ? OR clusters.cluster_uuid LIKE ?"
			query.Where(clusterQuery, value, value)
			continue
		}
		query.Where(key, value)
	}

	var count int64 = 0
	query.Count(&count)
	
	query.Order(orderQuery)
	
	err := query.Offset(offset).Limit(limit).Find(&recommendationSets).Error

	return recommendationSets, int(count), err
}

func (r *RecommendationSet) GetRecommendationSetByID(orgID string, recommendationID string) (RecommendationSet, error) {

	var recommendationSet RecommendationSet
	db := database.GetDB()

	db.Joins("JOIN workloads ON recommendation_sets.workload_id = workloads.id").
		Joins("JOIN clusters ON workloads.cluster_id = clusters.id").
		Joins("JOIN rh_accounts ON clusters.tenant_id = rh_accounts.id").
		Preload("Workload.Cluster.RHAccount").
		Where("rh_accounts.org_id = ?", orgID).
		Where("recommendation_sets.id = ?", recommendationID).
		First(&recommendationSet)

	return recommendationSet, nil
}

func (r *RecommendationSet) CreateRecommendationSet() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workload_id"}, {Name: "container_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"monitoring_start_time", "monitoring_end_time", "recommendations", "updated_at"}),
	}).Create(r)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func DeleteStaleRecommendationSet(workload_id uint, containers []string) error {
	db := database.GetDB()
	result := db.Where("workload_id = ? AND container_name NOT IN ?", workload_id, containers).Delete(&RecommendationSet{})
	if result.Error != nil {
		return result.Error
	}

	return nil
}
