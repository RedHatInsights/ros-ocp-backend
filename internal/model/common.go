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

func ptrFloat64(v float64) *float64 {
	return &v
}

// StoredVariationPcts holds pre-computed per-term, per-engine variation percentages fetched
// from DB columns. Used in API response building to avoid recomputing from the JSON blob.
// Fields are pointers to handle nullable DB columns (e.g. existing rows before migration).
type StoredVariationPcts struct {
	CPUVariationShortCostPct            *float64 `gorm:"column:cpu_variation_short_cost_pct" json:"-"`
	CPUVariationShortPerformancePct     *float64 `gorm:"column:cpu_variation_short_performance_pct" json:"-"`
	CPUVariationMediumCostPct           *float64 `gorm:"column:cpu_variation_medium_cost_pct" json:"-"`
	CPUVariationMediumPerformancePct    *float64 `gorm:"column:cpu_variation_medium_performance_pct" json:"-"`
	CPUVariationLongCostPct             *float64 `gorm:"column:cpu_variation_long_cost_pct" json:"-"`
	CPUVariationLongPerformancePct      *float64 `gorm:"column:cpu_variation_long_performance_pct" json:"-"`
	MemoryVariationShortCostPct         *float64 `gorm:"column:memory_variation_short_cost_pct" json:"-"`
	MemoryVariationShortPerformancePct  *float64 `gorm:"column:memory_variation_short_performance_pct" json:"-"`
	MemoryVariationMediumCostPct        *float64 `gorm:"column:memory_variation_medium_cost_pct" json:"-"`
	MemoryVariationMediumPerformancePct *float64 `gorm:"column:memory_variation_medium_performance_pct" json:"-"`
	MemoryVariationLongCostPct          *float64 `gorm:"column:memory_variation_long_cost_pct" json:"-"`
	MemoryVariationLongPerformancePct   *float64 `gorm:"column:memory_variation_long_performance_pct" json:"-"`
}

// HasValues reports whether at least one stored percentage is non-nil.
func (s *StoredVariationPcts) HasValues() bool {
	return s.CPUVariationShortCostPct != nil ||
		s.CPUVariationShortPerformancePct != nil ||
		s.CPUVariationMediumCostPct != nil ||
		s.CPUVariationMediumPerformancePct != nil ||
		s.CPUVariationLongCostPct != nil ||
		s.CPUVariationLongPerformancePct != nil ||
		s.MemoryVariationShortCostPct != nil ||
		s.MemoryVariationShortPerformancePct != nil ||
		s.MemoryVariationMediumCostPct != nil ||
		s.MemoryVariationMediumPerformancePct != nil ||
		s.MemoryVariationLongCostPct != nil ||
		s.MemoryVariationLongPerformancePct != nil
}

// Lookup returns the stored CPU and memory variation pct for a given term (e.g. "short_term") and
// engine name (e.g. "cost"). Returns (nil, nil) when the combination is not recognised.
func (s *StoredVariationPcts) Lookup(term, engine string) (cpu, mem *float64) {
	switch {
	case term == "short_term" && engine == "cost":
		return s.CPUVariationShortCostPct, s.MemoryVariationShortCostPct
	case term == "short_term" && engine == "performance":
		return s.CPUVariationShortPerformancePct, s.MemoryVariationShortPerformancePct
	case term == "medium_term" && engine == "cost":
		return s.CPUVariationMediumCostPct, s.MemoryVariationMediumCostPct
	case term == "medium_term" && engine == "performance":
		return s.CPUVariationMediumPerformancePct, s.MemoryVariationMediumPerformancePct
	case term == "long_term" && engine == "cost":
		return s.CPUVariationLongCostPct, s.MemoryVariationLongCostPct
	case term == "long_term" && engine == "performance":
		return s.CPUVariationLongPerformancePct, s.MemoryVariationLongPerformancePct
	}
	return nil, nil
}

// RecommendationColumnValues holds current request and per-term, per-engine variation
// as percent of current request. Values match transformComponentUnits + convertVariationToPercentage
// (CPU cores truncated to 3dp, memory bytes as MiB to 2dp, then percent to 3dp).
// Pointer fields are nil when a term/engine is absent so GORM persists SQL NULL.
// Used to populate recommendation_sets and namespace_recommendation_sets columns for sorting.
type RecommendationColumnValues struct {
	CPURequestCurrent    *float64
	MemoryRequestCurrent *float64

	CPUVariationShortCostPct            *float64
	CPUVariationShortPerformancePct     *float64
	CPUVariationMediumCostPct           *float64
	CPUVariationMediumPerformancePct    *float64
	CPUVariationLongCostPct             *float64
	CPUVariationLongPerformancePct      *float64
	MemoryVariationShortCostPct         *float64
	MemoryVariationShortPerformancePct  *float64
	MemoryVariationMediumCostPct        *float64
	MemoryVariationMediumPerformancePct *float64
	MemoryVariationLongCostPct          *float64
	MemoryVariationLongPerformancePct   *float64
}

// ExtractRecommendationColumnValues extracts current requests and per-term, per-engine
// variation as percent-of-request for recommendation_sets and namespace_recommendation_sets columns.
func ExtractRecommendationColumnValues(data kruizePayload.RecommendationData) RecommendationColumnValues {
	cpuReq := data.Current.Requests.Cpu.Amount
	memReq := data.Current.Requests.Memory.Amount

	// Clamp current request values to the backing DB numeric ranges to prevent insert/update failures.
	// cpu_request_current is NUMERIC(10,4); memory_request_current is NUMERIC(20,4).
	cpuReq = utils.ClampToNumeric10_4Range(cpuReq)
	memReq = utils.ClampToNumeric20_4Range(memReq)
	recommVals := RecommendationColumnValues{
		CPURequestCurrent:    ptrFloat64(cpuReq),
		MemoryRequestCurrent: ptrFloat64(memReq),
	}
	extractTermVariations(&recommVals, data.RecommendationTerms, cpuReq, memReq)
	return recommVals
}

func extractTermVariations(recommVals *RecommendationColumnValues, terms kruizePayload.Term, cpuReq, memReq float64) {
	if e := terms.Short_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationShortCostPct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Cost.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationShortCostPct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Cost.Variation.Requests.Memory.Amount, memReq))
		recommVals.CPUVariationShortPerformancePct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Performance.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationShortPerformancePct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Performance.Variation.Requests.Memory.Amount, memReq))
	}
	if e := terms.Medium_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationMediumCostPct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Cost.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationMediumCostPct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Cost.Variation.Requests.Memory.Amount, memReq))
		recommVals.CPUVariationMediumPerformancePct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Performance.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationMediumPerformancePct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Performance.Variation.Requests.Memory.Amount, memReq))
	}
	if e := terms.Long_term.RecommendationEngines; e != nil {
		recommVals.CPUVariationLongCostPct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Cost.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationLongCostPct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Cost.Variation.Requests.Memory.Amount, memReq))
		recommVals.CPUVariationLongPerformancePct = ptrFloat64(utils.VariationPercentOfRequestCPU(e.Performance.Variation.Requests.Cpu.Amount, cpuReq))
		recommVals.MemoryVariationLongPerformancePct = ptrFloat64(utils.VariationPercentOfRequestMemoryBytesMiB(e.Performance.Variation.Requests.Memory.Amount, memReq))
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
			"recommendation_sets.recommendations, "+
			"recommendation_sets.cpu_variation_short_cost_pct, "+
			"recommendation_sets.cpu_variation_short_performance_pct, "+
			"recommendation_sets.cpu_variation_medium_cost_pct, "+
			"recommendation_sets.cpu_variation_medium_performance_pct, "+
			"recommendation_sets.cpu_variation_long_cost_pct, "+
			"recommendation_sets.cpu_variation_long_performance_pct, "+
			"recommendation_sets.memory_variation_short_cost_pct, "+
			"recommendation_sets.memory_variation_short_performance_pct, "+
			"recommendation_sets.memory_variation_medium_cost_pct, "+
			"recommendation_sets.memory_variation_medium_performance_pct, "+
			"recommendation_sets.memory_variation_long_cost_pct, "+
			"recommendation_sets.memory_variation_long_performance_pct").
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
			"namespace_recommendation_sets.recommendations, "+
			"namespace_recommendation_sets.cpu_variation_short_cost_pct, "+
			"namespace_recommendation_sets.cpu_variation_short_performance_pct, "+
			"namespace_recommendation_sets.cpu_variation_medium_cost_pct, "+
			"namespace_recommendation_sets.cpu_variation_medium_performance_pct, "+
			"namespace_recommendation_sets.cpu_variation_long_cost_pct, "+
			"namespace_recommendation_sets.cpu_variation_long_performance_pct, "+
			"namespace_recommendation_sets.memory_variation_short_cost_pct, "+
			"namespace_recommendation_sets.memory_variation_short_performance_pct, "+
			"namespace_recommendation_sets.memory_variation_medium_cost_pct, "+
			"namespace_recommendation_sets.memory_variation_medium_performance_pct, "+
			"namespace_recommendation_sets.memory_variation_long_cost_pct, "+
			"namespace_recommendation_sets.memory_variation_long_performance_pct").
		Joins(`
			JOIN workloads ON namespace_recommendation_sets.workload_id = workloads.id
			JOIN clusters ON workloads.cluster_id = clusters.id
		`).Model(&NamespaceRecommendationSetResult{}).
		Where("namespace_recommendation_sets.org_id = ?", orgID)
	return query
}
