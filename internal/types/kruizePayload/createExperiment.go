package kruizePayload

import (
	"encoding/json"
)

type createExperiment struct {
	Version                 string                 `json:"version"`
	Experiment_name         string                 `json:"experiment_name"`
	Cluster_name            string                 `json:"cluster_name"`
	Performance_profile     string                 `json:"performance_profile"`
	Mode                    string                 `json:"mode"`
	Target_cluster          string                 `json:"target_cluster"`
	Kubernetes_objects      []kubernetesObject     `json:"kubernetes_objects"`
	Trial_settings          TrialSettings          `json:"trial_settings"`
	Recommendation_settings RecommendationSettings `json:"recommendation_settings"`
}

type TrialSettings struct {
	Measurement_duration string `json:"measurement_duration"`
}

type RecommendationSettings struct {
	Threshold string `json:"threshold"`
}

func GetCreateExperimentPayload(experiment_name string, cluster_identifier string, containers []map[string]string, data map[string]string) ([]byte, error) {
	container_array := []container{}
	for _, c := range containers {
		container_array = append(container_array, container{
			Container_image_name: c["container_image_name"],
			Container_name:       c["container_name"],
		})
	}
	payload := []createExperiment{
		{
			Version:                 "1.0", // TODO To be set to cfg.KruizePerformanceProfileVersion
			Experiment_name:         experiment_name,
			Cluster_name:            cluster_identifier,
			Performance_profile:     "resource-optimization-openshift",
			Mode:                    "monitor",
			Target_cluster:          "remote",
			Trial_settings:          TrialSettings{Measurement_duration: "15min"},
			Recommendation_settings: RecommendationSettings{Threshold: "0.1"},
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
