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
	KubernetesObjects      []NamespaceKubernetesObject          `json:"kubernetes_objects"`
	TrialSettings          kruizePayload.TrialSettings          `json:"trial_settings"`
	RecommendationSettings kruizePayload.RecommendationSettings `json:"recommendation_settings"`
}
