package namespace

import (
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

type CreateNamespaceExperiment struct {
	Version                string                               `json:"version"`
	ExperimentName         string                               `json:"experiment_name"`
	ClusterName            string                               `json:"cluster_name"`
	PerformanceProfile     string                               `json:"performance_profile"`
	Mode                   string                               `json:"mode"`
	TargetCluster          string                               `json:"target_cluster"`
	ExperimentType         string                               `json:"experiment_type"`
	KubernetesObjects      []NamespaceKubernetesObject          `json:"kubernetes_objects"`
	TrialSettings          kruizePayload.TrialSettings          `json:"trial_settings"`
	RecommendationSettings kruizePayload.RecommendationSettings `json:"recommendation_settings"`
}

func GetCreateNamespaceExperimentPayload(experiment_name string, cluster_identifier string, namespace string) []CreateNamespaceExperiment {
	return []CreateNamespaceExperiment{
		{
			Version:            "v2.0",
			ExperimentName:     experiment_name,
			ClusterName:        cluster_identifier,
			PerformanceProfile: "resource-optimization-openshift",
			Mode:               "monitor",
			TargetCluster:      "remote",
			ExperimentType:     "namespace",
			KubernetesObjects: []NamespaceKubernetesObject{
				{
					Namespaces: NamespaceObject{Namespace: namespace},
				},
			},
			TrialSettings:          kruizePayload.TrialSettings{Measurement_duration: "15min"},
			RecommendationSettings: kruizePayload.RecommendationSettings{Threshold: "0.1"},
		},
	}
}
