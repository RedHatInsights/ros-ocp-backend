package kruizePayload

type UpdateResult struct {
	Version             string             `json:"version"`
	Experiment_name     string             `json:"experiment_name"`
	Interval_start_time string             `json:"interval_start_time"`
	Interval_end_time   string             `json:"interval_end_time"`
	Kubernetes_objects  []kubernetesObject `json:"kubernetes_objects"`
}

func GetUpdateResultPayload(experiment_name string, containers []map[string]interface{}, data map[string]string) []UpdateResult {
	container_array := []container{}
	for _, c := range containers {
		container_array = append(container_array, make_container_data(c))
	}
	payload := []UpdateResult{
		{
			Version:             "1.0",
			Experiment_name:     experiment_name,
			Interval_start_time: data["interval_start"],
			Interval_end_time:   data["interval_end"],
			Kubernetes_objects: []kubernetesObject{
				{
					K8stype:    data["k8s_object_type"],
					Name:       data["k8s_object_name"],
					Namespace:  data["namespace"],
					Containers: container_array,
				},
			},
		},
	}
	return payload
}
