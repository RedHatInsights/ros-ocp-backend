package kruizePayload

type ListRecommendations struct {
	Cluster_name       string             `json:"cluster_name,omitempty"`
	Experiment_name    string             `json:"experiment_name,omitempty"`
	Version            string             `json:"version,omitempty"`
	Kubernetes_objects []kubernetesObject `json:"kubernetes_objects,omitempty"`
}
