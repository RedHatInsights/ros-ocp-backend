package namespace

import (
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

type UpdateNamespaceResultResponse struct {
	Message           string                             `json:"message,omitempty"`
	Httpcode          int                                `json:"httpcode,omitempty"`
	DocumentationLink string                             `json:"documentationLink,omitempty"`
	Status            string                             `json:"status,omitempty"`
	Data              []kruizePayload.UpdateResponseData `json:"data,omitempty"`
}

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

func makeNamespaceMetrics(row map[string]any) []kruizePayload.Metric {
	namespaceMetrics := []kruizePayload.Metric{}

	namespaceMetricsMap := map[string]map[string]string{

		// w.r.t namespace request and limit are additive
		// avg, min, max are not relevant for the same
		"namespaceCpuRequest": {
			"sum":    "cpu_request_namespace_sum_SUM",
			"format": "cores",
		},
		"namespaceCpuLimit": {
			"sum":    "cpu_limit_namespace_sum_SUM",
			"format": "cores",
		},
		"namespaceCpuUsage": {
			"avg":    "cpu_usage_namespace_avg_MEAN",
			"min":    "cpu_usage_namespace_min_MIN",
			"max":    "cpu_usage_namespace_max_MAX",
			"format": "cores",
		},
		"namespaceCpuThrottle": {
			"avg":    "cpu_throttle_namespace_avg_MEAN",
			"min":    "cpu_throttle_namespace_min_MIN",
			"max":    "cpu_throttle_namespace_max_MAX",
			"format": "cores",
		},
		"namespaceMemoryRequest": {
			"sum":    "memory_request_namespace_sum_SUM",
			"format": "bytes",
		},
		"namespaceMemoryLimit": {
			"sum":    "memory_limit_namespace_sum_SUM",
			"format": "bytes",
		},
		"namespaceMemoryUsage": {
			"avg":    "memory_usage_namespace_avg_MEAN",
			"min":    "memory_usage_namespace_min_MIN",
			"max":    "memory_usage_namespace_max_MAX",
			"format": "bytes",
		},
		"namespaceMemoryRSS": {
			"avg":    "memory_rss_usage_namespace_avg_MEAN",
			"min":    "memory_rss_usage_namespace_min_MIN",
			"max":    "memory_rss_usage_namespace_max_MAX",
			"format": "bytes",
		},
		"namespaceTotalPods": {
			"avg": "namespace_total_pods_avg_MEAN",
			"max": "namespace_total_pods_max_MAX",
		},
		"namespaceRunningPods": {
			"avg": "namespace_running_pods_avg_MEAN",
			"max": "namespace_running_pods_max_MAX",
		},
	}

	for metricName, metricFields := range namespaceMetricsMap {
		sum, avg, min, max, format := "", "", "", "", ""

		if field, ok := metricFields["sum"]; ok {
			sum = kruizePayload.AssertAndConvertToString(row[field])
		}
		if field, ok := metricFields["avg"]; ok {
			avg = kruizePayload.AssertAndConvertToString(row[field])
		}
		if field, ok := metricFields["min"]; ok {
			min = kruizePayload.AssertAndConvertToString(row[field])
		}
		if field, ok := metricFields["max"]; ok {
			max = kruizePayload.AssertAndConvertToString(row[field])
		}
		if field, ok := metricFields["format"]; ok {
			format = field
		}

		hasValue := sum != "" || avg != "" || min != "" || max != ""

		if hasValue {
			namespaceMetrics = append(namespaceMetrics, kruizePayload.Metric{
				Name: metricName,
				Results: kruizePayload.Result{
					Aggregation_info: kruizePayload.AggregatedData{
						Sum:    sum,
						Avg:    avg,
						Min:    min,
						Max:    max,
						Format: format,
					},
				},
			})
		}
	}

	return namespaceMetrics
}
