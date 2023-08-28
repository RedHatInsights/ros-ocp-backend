package kruizePayload

import (
	"github.com/go-gota/gota/dataframe"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
)

type UpdateResult struct {
	Version             string             `json:"version"`
	Experiment_name     string             `json:"experiment_name"`
	Interval_start_time string             `json:"interval_start_time"`
	Interval_end_time   string             `json:"interval_end_time"`
	Kubernetes_objects  []kubernetesObject `json:"kubernetes_objects"`
}

func GetUpdateResultPayload(experiment_name string, containers []map[string]interface{}) []UpdateResult {
	payload := []UpdateResult{}
	df := dataframe.LoadMaps(containers)
	for _, v := range df.GroupBy("interval_end").GetGroups() {
		k8s_object := v.Maps()
		data := map[string]string{
			"namespace":       k8s_object[0]["namespace"].(string),
			"k8s_object_type": k8s_object[0]["k8s_object_type"].(string),
			"k8s_object_name": k8s_object[0]["k8s_object_name"].(string),
			"interval_start":  utils.ConvertDateToISO8601(k8s_object[0]["interval_start"].(string)),
			"interval_end":    utils.ConvertDateToISO8601(k8s_object[0]["interval_end"].(string)),
		}
		container_array := []container{}
		for _, c := range k8s_object {
			container_array = append(container_array, make_container_data(c))
		}
		payload = append(payload, UpdateResult{
			Version:             "3.0",
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
		})
	}

	return payload
}
