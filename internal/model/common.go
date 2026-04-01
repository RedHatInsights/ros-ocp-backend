package model

import (
	"gorm.io/gorm"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

const (
	// NamespaceMaxLen is the max length for namespace/project names (K8s RFC 1123 DNS label).
	NamespaceMaxLen = 63
	// ClusterMaxLen is the max length for cluster alias (K8s DNS subdomain).
	ClusterMaxLen = 253
)

// RecommendationColumnValues holds current request and per-term, per-engine variation
// amounts extracted from recommendation data. Used to populate DB columns on
// recommendation_sets and namespace_recommendation_sets for sorting/filtering.
type RecommendationColumnValues struct {
	CPURequestCurrent    float64
	MemoryRequestCurrent float64

	CPUVariationShortCost            float64
	CPUVariationShortPerformance     float64
	CPUVariationMediumCost           float64
	CPUVariationMediumPerformance    float64
	CPUVariationLongCost             float64
	CPUVariationLongPerformance      float64
	MemoryVariationShortCost         float64
	MemoryVariationShortPerformance  float64
	MemoryVariationMediumCost        float64
	MemoryVariationMediumPerformance float64
	MemoryVariationLongCost          float64
	MemoryVariationLongPerformance   float64
}

// ExtractRecommendationColumnValues extracts current requests and per-term, per-engine
// variation amounts from RecommendationData. Reusable for both namespace and container
// recommendation flows.
func ExtractRecommendationColumnValues(data kruizePayload.RecommendationData) RecommendationColumnValues {
	recommVals := RecommendationColumnValues{
		CPURequestCurrent:    data.Current.Requests.Cpu.Amount,
		MemoryRequestCurrent: data.Current.Requests.Memory.Amount,
	}
	extractTermVariations(&recommVals, data.RecommendationTerms)
	return recommVals
}

func extractTermVariations(recommVals *RecommendationColumnValues, terms kruizePayload.Term) {
	if e := terms.Short_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationShortCost = e.Cost.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationShortCost = e.Cost.Variation.Requests.Memory.Amount
		recommVals.CPUVariationShortPerformance = e.Performance.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationShortPerformance = e.Performance.Variation.Requests.Memory.Amount
	}
	if e := terms.Medium_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationMediumCost = e.Cost.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationMediumCost = e.Cost.Variation.Requests.Memory.Amount
		recommVals.CPUVariationMediumPerformance = e.Performance.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationMediumPerformance = e.Performance.Variation.Requests.Memory.Amount
	}
	if e := terms.Long_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationLongCost = e.Cost.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationLongCost = e.Cost.Variation.Requests.Memory.Amount
		recommVals.CPUVariationLongPerformance = e.Performance.Variation.Requests.Cpu.Amount
		recommVals.MemoryVariationLongPerformance = e.Performance.Variation.Requests.Memory.Amount
	}
}

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
			"namespace_recommendation_sets.recommendations").
		Joins(`
			JOIN workloads ON namespace_recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
		`).Model(&NamespaceRecommendationSetResult{}).
		Where("namespace_recommendation_sets.org_id = ?", orgID)
	return query
}
