package model

import (
	"gorm.io/gorm"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
)

const (
	// NamespaceMaxLen is the max length for namespace/project names (K8s RFC 1123 DNS label).
	NamespaceMaxLen = 63
	// ClusterMaxLen is the max length for cluster alias (K8s DNS subdomain).
	ClusterMaxLen = 253
)

// RecommendationColumnValues holds current request and per-term, per-engine variation
// as percent of current request (utils.CalculatePercentage, same as convertVariationToPercentage for requests).
// Used to populate namespace_recommendation_sets columns for sorting aligned with API display.
type RecommendationColumnValues struct {
	CPURequestCurrent    float64
	MemoryRequestCurrent float64

	CPUVariationShortCostPct            float64
	CPUVariationShortPerformancePct     float64
	CPUVariationMediumCostPct           float64
	CPUVariationMediumPerformancePct    float64
	CPUVariationLongCostPct             float64
	CPUVariationLongPerformancePct      float64
	MemoryVariationShortCostPct         float64
	MemoryVariationShortPerformancePct  float64
	MemoryVariationMediumCostPct        float64
	MemoryVariationMediumPerformancePct float64
	MemoryVariationLongCostPct          float64
	MemoryVariationLongPerformancePct   float64
}

// ExtractRecommendationColumnValues extracts current requests and per-term, per-engine
// variation as percent-of-request for namespace_recommendation_sets columns (namespace poller).
func ExtractRecommendationColumnValues(data kruizePayload.RecommendationData) RecommendationColumnValues {
	recommVals := RecommendationColumnValues{
		CPURequestCurrent:    data.Current.Requests.Cpu.Amount,
		MemoryRequestCurrent: data.Current.Requests.Memory.Amount,
	}
	extractTermVariations(&recommVals, data.RecommendationTerms)
	return recommVals
}

func extractTermVariations(recommVals *RecommendationColumnValues, terms kruizePayload.Term) {
	cpuReq := recommVals.CPURequestCurrent
	memReq := recommVals.MemoryRequestCurrent

	if e := terms.Short_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationShortCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationShortCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Memory.Amount, memReq)
		recommVals.CPUVariationShortPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationShortPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Memory.Amount, memReq)
	}
	if e := terms.Medium_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationMediumCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationMediumCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Memory.Amount, memReq)
		recommVals.CPUVariationMediumPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationMediumPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Memory.Amount, memReq)
	}
	if e := terms.Long_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationLongCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationLongCostPct = utils.CalculatePercentage(e.Cost.Variation.Requests.Memory.Amount, memReq)
		recommVals.CPUVariationLongPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Cpu.Amount, cpuReq)
		recommVals.MemoryVariationLongPerformancePct = utils.CalculatePercentage(e.Performance.Variation.Requests.Memory.Amount, memReq)
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
