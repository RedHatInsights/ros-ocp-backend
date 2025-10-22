package model

import (
	"gorm.io/gorm"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

func getRecommendationQuery(orgID string) *gorm.DB {
	db := database.GetDB()
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
	return query
}

func getNamespaceRecommendationQuery(orgID string) *gorm.DB {
	db := database.GetDB()
	query := db.Table("namespace_recommendation_sets").
		Select("namespace_recommendation_sets.id, "+
			"namespace_recommendation_sets.namespace_name AS project, "+
			"clusters.source_id, "+
			"clusters.cluster_uuid, "+
			"clusters.cluster_alias, "+
			"clusters.last_reported_at AS last_reported, "+
			"namespace_recommendation_sets.cpu_request_current, "+
			"namespace_recommendation_sets.cpu_variation, "+
			"namespace_recommendation_sets.memory_request_current, "+
			"namespace_recommendation_sets.memory_variation, "+
			"namespace_recommendation_sets.recommendations").
		Joins(`
			JOIN workloads ON namespace_recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
		`).Model(&NamespaceRecommendationSetResult{}).
		Where("workloads.org_id = ?", orgID)
	return query
}
