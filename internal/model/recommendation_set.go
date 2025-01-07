package model

import (
	"fmt"
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

type RecommendationSetResult struct {
	/*
		Intended to be an API-ready struct
		Updated recommendation data is saved to RecommendationsJSON
		Before the API response is sent
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
}

func (r *RecommendationSet) AfterFind(tx *gorm.DB) error {
	r.MonitoringEndTimeStr = r.MonitoringEndTime.Format(time.RFC3339)
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

func (r *RecommendationSet) GetRecommendationSet(orgID string, user_permissions map[string][]string, opts ...GetRecommendationOptions,
) ([]RecommendationSetResult, int, error) {
	db := database.GetDB()
	var recommendationSet []RecommendationSetResult

	/*
		In case RecommendationID is provided other values are not expected
		Other values have defaults as well
	*/
	var options GetRecommendationOptions
	if len(opts) > 0 {
		options = opts[0]
	} else {
		err := fmt.Errorf("missing GetRecommendationOptions")
		return recommendationSet, 0, err
	}

	query := db.Table("recommendation_sets").
		Select("recommendation_sets.id, "+
			"recommendation_sets.container_name AS container, "+
			"workloads.namespace AS project, "+
			"workloads.workload_name as workload, "+
			"workloads.workload_type, "+
			"clusters.source_id, "+
			"clusters.cluster_uuid, "+
			"clusters.cluster_alias, "+
			"clusters.last_reported_at AS last_reported, "+
			"recommendation_sets.recommendations").
		Joins(`
			JOIN workloads ON recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
			JOIN rh_accounts ON clusters.tenant_id = rh_accounts.id
		`).Model(&RecommendationSetResult{}).
		Where("rh_accounts.org_id = ?", orgID)

	add_rbac_filter(query, user_permissions)

	if options.RecommendationID != "" {
		query.Where("recommendation_sets.id = ?", options.RecommendationID)
		err := query.First(&recommendationSet).Error
		return recommendationSet, int(1), err
	} else {
		for key, values := range options.QueryParams {
			switch v := values.(type) {
			case []string:
				// Convert []string to []interface{} for unpacking below
				args := make([]interface{}, len(v))
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
		query.Order(options.OrderQuery)
		err := query.Offset(options.Offset).Limit(options.Limit).Scan(&recommendationSet).Error
		return recommendationSet, int(count), err
	}
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
