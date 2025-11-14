package namespace

import (
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

//nolint:unused
type UpdateNamespaceResultResponse struct {
	Message           string                             `json:"message,omitempty"`
	Httpcode          int                                `json:"httpcode,omitempty"`
	DocumentationLink string                             `json:"documentationLink,omitempty"`
	Status            string                             `json:"status,omitempty"`
	Data              []kruizePayload.UpdateResponseData `json:"data,omitempty"`
}

//nolint:unused
type NamespaceRecommendationResponse []NamespaceExperiment

type NamespaceExperiment struct {
	ClusterName       string                      `json:"cluster_name,omitempty"`
	ExperimentType    string                      `json:"experiment_type,omitempty"`
	KubernetesObjects []NamespaceKubernetesObject `json:"kubernetes_objects,omitempty"`
	Version           string                      `json:"version,omitempty"`
	ExperimentName    string                      `json:"experiment_name,omitempty"`
}

type NamespaceKubernetesObject struct {
	Namespace  string          `json:"namespace,omitempty"`
	Containers []any           `json:"containers,omitempty"`
	Namespaces NamespaceObject `json:"namespaces"`
}

type NamespaceObject struct {
	Namespace       string                       `json:"namespace,omitempty"`
	Recommendations kruizePayload.Recommendation `json:"recommendations"`
}
