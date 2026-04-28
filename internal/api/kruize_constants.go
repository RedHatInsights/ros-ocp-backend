package api

const (
	// Canonical Kruize recommendation term keys as they appear in the JSON payload.
	KruizeShortTerm  = "short_term"
	KruizeMediumTerm = "medium_term"
	KruizeLongTerm   = "long_term"

	// Canonical Kruize recommendation engine keys as they appear in the JSON payload.
	KruizeEngineCost        = "cost"
	KruizeEnginePerformance = "performance"
)

// Keep ordering stable for deterministic iteration.
var kruizeRecommendationTerms = []string{KruizeShortTerm, KruizeMediumTerm, KruizeLongTerm}

// Keep ordering stable for deterministic iteration.
var kruizeRecommendationEngines = []string{KruizeEngineCost, KruizeEnginePerformance}
