package namespace

import (
	"github.com/go-gota/gota/dataframe"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	kruizePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
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

func GetUpdateNamespaceResultPayload(experiment_name string, namespaceData []map[string]any) []UpdateNamespaceResult {
	cfg := config.GetConfig()

	payload := []UpdateNamespaceResult{}
	df := dataframe.LoadMaps(
		namespaceData,
		dataframe.WithTypes(types.NamespaceCSVColumnMapping),
	)
	for _, v := range df.GroupBy("interval_end").GetGroups() {
		k8s_object := v.Maps()
		row := k8s_object[0]

		namespace := kruizePayload.AssertAndConvertToString(row["namespace"])
		intervalStart := utils.ConvertDateToISO8601(row["interval_start"].(string))
		intervalEnd := utils.ConvertDateToISO8601(row["interval_end"].(string))

		metrics := makeNamespaceMetrics(row)

		payload = append(payload, UpdateNamespaceResult{
			Version:           cfg.KruizePerformanceProfileVersion,
			ExperimentName:    experiment_name,
			IntervalStartTime: intervalStart,
			IntervalEndTime:   intervalEnd,
			KubernetesObjects: []NamespaceK8SObjectUpdateResult{
				{
					Namespaces: NamespaceMetrics{
						Namespace: namespace,
						Metrics:   metrics,
					},
				},
			},
		})
	}
	return payload
}
