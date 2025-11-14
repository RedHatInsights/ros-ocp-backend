package namespace

//nolint:unused
type UpdateNamespaceResult struct {
	Version             string                      `json:"version"`
	Experiment_name     string                      `json:"experiment_name"`
	Interval_start_time string                      `json:"interval_start_time"`
	Interval_end_time   string                      `json:"interval_end_time"`
	Kubernetes_objects  []NamespaceKubernetesObject `json:"kubernetes_objects"`
}
