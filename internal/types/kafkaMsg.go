package types

import "time"

type KafkaMsg struct {
	Request_id   string `validate:"required"`
	B64_identity string `validate:"required"`
	Metadata     struct {
		Account       string
		Org_id        string `validate:"required"`
		Source_id     string `validate:"required"`
		Cluster_uuid  string `validate:"required"`
		Cluster_alias string `validate:"required"`
	} `validate:"required"`
	Files []string `validate:"required"`
}

type RecommendationMetadata struct {
	Org_id             string    `validate:"required"`
	Workload_id        uint      `validate:"required"`
	Experiment_name    string    `validate:"required"`
	Max_endtime_report time.Time `validate:"required"`
}

type RecommendationKafkaMsg struct {
	Request_id string                 `validate:"required"`
	Metadata   RecommendationMetadata `validate:"required"`
}
