package kruize

import (
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
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

// ExtractRecommendationColumnValues reads current requests and per-term variation
// amounts from a RecommendationData entry. Returns zero values for any missing
// terms or engines. Reusable for both namespace and container recommendation flows.
func ExtractRecommendationColumnValues(data kruizePayload.RecommendationData) RecommendationColumnValues {
	cols := RecommendationColumnValues{
		CPURequestCurrent:    data.Current.Requests.Cpu.Amount,
		MemoryRequestCurrent: data.Current.Requests.Memory.Amount,
	}

	extractTermVariations(&cols, data.RecommendationTerms)
	return cols
}

func extractTermVariations(cols *RecommendationColumnValues, terms kruizePayload.Term) {
	if e := terms.Short_term.RecommendationEngines; e != nil {
		cols.CPUVariationShortCost = e.Cost.Variation.Requests.Cpu.Amount
		cols.MemoryVariationShortCost = e.Cost.Variation.Requests.Memory.Amount
		cols.CPUVariationShortPerformance = e.Performance.Variation.Requests.Cpu.Amount
		cols.MemoryVariationShortPerformance = e.Performance.Variation.Requests.Memory.Amount
	}

	if e := terms.Medium_term.RecommendationEngines; e != nil {
		cols.CPUVariationMediumCost = e.Cost.Variation.Requests.Cpu.Amount
		cols.MemoryVariationMediumCost = e.Cost.Variation.Requests.Memory.Amount
		cols.CPUVariationMediumPerformance = e.Performance.Variation.Requests.Cpu.Amount
		cols.MemoryVariationMediumPerformance = e.Performance.Variation.Requests.Memory.Amount
	}

	if e := terms.Long_term.RecommendationEngines; e != nil {
		cols.CPUVariationLongCost = e.Cost.Variation.Requests.Cpu.Amount
		cols.MemoryVariationLongCost = e.Cost.Variation.Requests.Memory.Amount
		cols.CPUVariationLongPerformance = e.Performance.Variation.Requests.Cpu.Amount
		cols.MemoryVariationLongPerformance = e.Performance.Variation.Requests.Memory.Amount
	}
}
