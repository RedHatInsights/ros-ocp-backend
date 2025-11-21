package namespace

import (
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

type NamespaceMetrics struct {
	Namespace string                 `json:"namespace,omitempty"`
	Metrics   []kruizePayload.Metric `json:"metrics"`
}

type NamespaceK8SObjectUpdateResult struct {
	Namespaces NamespaceMetrics `json:"namespaces"`
}

type UpdateNamespaceResult struct {
	Version           string                           `json:"version"`
	ExperimentName    string                           `json:"experiment_name"`
	IntervalStartTime string                           `json:"interval_start_time"`
	IntervalEndTime   string                           `json:"interval_end_time"`
	KubernetesObjects []NamespaceK8SObjectUpdateResult `json:"kubernetes_objects"`
}
