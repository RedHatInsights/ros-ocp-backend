package kruizePayload

import (
	"encoding/json"
)

type createExperiment struct {
	Version                 string                  `json:"version"`
	Experiment_name         string                  `json:"experiment_name"`
	Performance_profile     string                  `json:"performance_profile"`
	Mode                    string                  `json:"mode"`
	Target_cluster          string                  `json:"target_cluster"`
	Kubernetes_objects      []kubernetesObject      `json:"kubernetes_objects"`
	Trial_settings          trial_settings          `json:"trial_settings"`
	Recommendation_settings recommendation_settings `json:"recommendation_settings"`
}

type trial_settings struct {
	Measurement_duration string `json:"measurement_duration"`
}

type recommendation_settings struct {
	Threshold string `json:"threshold"`
}

func GetCreateExperimentPayload(experiment_name string, containers []map[string]string, data map[string]string) ([]byte, error) {
	container_array := []container{}
	for _, c := range containers {
		container_array = append(container_array, container{
			Container_image_name: c["container_image_name"],
			Container_name:       c["container_name"],
		})
	}
	payload := []createExperiment{
		{
			Version:                 "1.0",
			Experiment_name:         experiment_name,
			Performance_profile:     "resource-optimization-openshift",
			Mode:                    "monitor",
			Target_cluster:          "remote",
			Trial_settings:          trial_settings{Measurement_duration: "15min"},
			Recommendation_settings: recommendation_settings{Threshold: "0.1"},
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

	postBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return postBody, nil
}
