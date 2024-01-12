package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
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

func GetFirstRecommendationSetsByWorkloadID(workload_id uint) (RecommendationSet, error) {
	recommendationSets := RecommendationSet{}
	db := database.GetDB()
	query := db.Where("workload_id = ?", workload_id).First(&recommendationSets)
	if query.Error != nil && query.Error.Error() == "record not found" {
		return recommendationSets, nil
	}
	return recommendationSets, query.Error
}

func (r *RecommendationSet) GetRecommendationSets(orgID string, orderQuery string, limit int, offset int, queryParams map[string][]string, user_permissions map[string][]string) ([]RecommendationSet, int, error) {

	var recommendationSets []RecommendationSet
	db := database.GetDB()

	query := db.Table("recommendation_sets").Joins(`
			JOIN workloads ON recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
			JOIN rh_accounts ON clusters.tenant_id = rh_accounts.id
		`).Model(r).Preload("Workload.Cluster.RHAccount").Where("rh_accounts.org_id = ?", orgID)

	add_rbac_filter(query, user_permissions)

	for key, values := range queryParams {
		valuesInterface := make([]interface{}, len(values))
		for i, v := range values {
			valuesInterface[i] = v
		}
		query.Where(key, valuesInterface...)
	}

	var count int64 = 0
	query.Count(&count)
	query.Order(orderQuery)
	err := query.Offset(offset).Limit(limit).Find(&recommendationSets).Error

	return recommendationSets, int(count), err
}

func (r *RecommendationSet) GetRecommendationSetByID(orgID string, recommendationID string, user_permissions map[string][]string) (RecommendationSet, error) {

	var recommendationSet RecommendationSet
	db := database.GetDB()

	query := db.Joins("JOIN workloads ON recommendation_sets.workload_id = workloads.id").
		Joins("JOIN clusters ON workloads.cluster_id = clusters.id").
		Joins("JOIN rh_accounts ON clusters.tenant_id = rh_accounts.id").
		Preload("Workload.Cluster.RHAccount").
		Where("rh_accounts.org_id = ?", orgID).
		Where("recommendation_sets.id = ?", recommendationID)

	add_rbac_filter(query, user_permissions)
	query.First(&recommendationSet)

	return recommendationSet, nil
}

func (r *RecommendationSet) CreateRecommendationSet(tx *gorm.DB) error {
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workload_id"}, {Name: "container_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"monitoring_start_time", "monitoring_end_time", "recommendations", "updated_at"}),
	}).Create(r)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}

func add_rbac_filter(query *gorm.DB, user_permissions map[string][]string) {
	cfg := config.GetConfig()
	if cfg.RBACEnabled {
		if _, ok := user_permissions["*"]; ok {
			return
		}

		if cluster_permissions, ok := user_permissions["openshift.cluster"]; ok {
			if project_permissions, ok := user_permissions["openshift.project"]; ok {
				if utils.StringInSlice("*", cluster_permissions) && utils.StringInSlice("*", project_permissions) {
					return
				} else if utils.StringInSlice("*", cluster_permissions) {
					query.Where("workloads.namespace IN (?)", project_permissions)
					return
				} else if utils.StringInSlice("*", project_permissions) {
					query.Where("clusters.cluster_uuid IN (?)", cluster_permissions)
					return
				} else {
					query.Where("clusters.cluster_uuid IN (?)", cluster_permissions)
					query.Where("workloads.namespace IN (?)", project_permissions)
					return
				}
			}
		}

		// if user has cluster level permision but project level permissions is not explicitly set
		// that means user have access to all projects in that cluster
		if cluster_permissions, ok := user_permissions["openshift.cluster"]; ok {
			if _, ok := user_permissions["openshift.project"]; !ok {
				if !utils.StringInSlice("*", cluster_permissions) {
					query.Where("clusters.cluster_uuid IN (?)", cluster_permissions)
					return
				}
			}
		}

		// if user has project level permision but cluster level permissions is not explicitly set
		// that means user have access to project in all the clusters
		if _, ok := user_permissions["openshift.cluster"]; !ok {
			if project_permissions, ok := user_permissions["openshift.project"]; ok {
				if !utils.StringInSlice("*", project_permissions) {
					query.Where("workloads.namespace IN (?)", project_permissions)
					return
				}
			}
		}
	}
}
