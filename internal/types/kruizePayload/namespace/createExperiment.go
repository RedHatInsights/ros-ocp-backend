package namespace

import (
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

//nolint:unused
type CreateNamespaceExperiment struct {
	Version                 string                               `json:"version"`
	Experiment_name         string                               `json:"experiment_name"`
	Cluster_name            string                               `json:"cluster_name"`
	Performance_profile     string                               `json:"performance_profile"`
	Mode                    string                               `json:"mode"`
	Target_cluster          string                               `json:"target_cluster"`
	Kubernetes_objects      []NamespaceKubernetesObject          `json:"kubernetes_objects"`
	Trial_settings          kruizePayload.TrialSettings          `json:"trial_settings"`
	Recommendation_settings kruizePayload.RecommendationSettings `json:"recommendation_settings"`
}
