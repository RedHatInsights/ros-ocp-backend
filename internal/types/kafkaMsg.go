package types

type KafkaMsg struct {
	Request_id   string `validate:"required"`
	B64_identity string `validate:"required"`
	Metadata     struct {
		Account       string `validate:"required"`
		Org_id        string `validate:"required"`
		Source_id     string `validate:"required"`
		Cluster_uuid  string `validate:"required"`
		Cluster_alias string `validate:"required"`
	} `validate:"required,dive"`
	Files []string `validate:"required"`
}
