package model

type GetRecommendationOptions struct {
	OrderQuery       string
	Limit            int
	Offset           int
	QueryParams      map[string]interface{}
	RecommendationID string
}
