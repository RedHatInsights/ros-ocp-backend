package kruizePayload

import (
	"github.com/go-gota/gota/dataframe"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
)

type UpdateResult struct {
	Version             string             `json:"version"`
	Experiment_name     string             `json:"experiment_name"`
	Interval_start_time string             `json:"interval_start_time"`
	Interval_end_time   string             `json:"interval_end_time"`
	Kubernetes_objects  []kubernetesObject `json:"kubernetes_objects"`
}

type UpdateResultResponse struct {
	Message           string               `json:"message,omitempty"`
	Httpcode          int                  `json:"httpcode,omitempty"`
	DocumentationLink string               `json:"documentationLink,omitempty"`
	Status            string               `json:"status,omitempty"`
	Data              []UpdateResponseData `json:"data,omitempty"`
}

type UpdateResponseData struct {
	Interval_start_time string
	Interval_end_time   string
	Errors              []ErrorData `json:"errors,omitempty"`
}

type ErrorData struct {
	Message           string `json:"message,omitempty"`
	Httpcode          int    `json:"httpcode,omitempty"`
	DocumentationLink string `json:"documentationLink,omitempty"`
	Status            string `json:"status,omitempty"`
}

func GetUpdateResultPayload(experiment_name string, containers []map[string]interface{}) []UpdateResult {
	payload := []UpdateResult{}
	df := dataframe.LoadMaps(
		containers,
		dataframe.WithTypes(types.CSVColumnMapping),
	)
	log := logging.GetLogger()
	for _, v := range df.GroupBy("interval_end").GetGroups() {
		k8s_object := v.Maps()
		ns := AssertAndConvertToString(k8s_object[0]["namespace"])
		objType := k8s_object[0]["k8s_object_type"].(string)
		objName := k8s_object[0]["k8s_object_name"].(string)
		intervalStart, err := utils.ConvertDateToISO8601(k8s_object[0]["interval_start"].(string))
		if err != nil {
			log.Errorf("skipping group (namespace=%s, %s/%s): %v", ns, objType, objName, err)
			continue
		}
		intervalEnd, err := utils.ConvertDateToISO8601(k8s_object[0]["interval_end"].(string))
		if err != nil {
			log.Errorf("skipping group (namespace=%s, %s/%s): %v", ns, objType, objName, err)
			continue
		}
		data := map[string]string{
			"namespace":       ns,
			"k8s_object_type": objType,
			"k8s_object_name": objName,
			"interval_start":  intervalStart,
			"interval_end":    intervalEnd,
		}
		container_array := []container{}
		for _, c := range k8s_object {
			container_array = append(container_array, make_container_data(c))
		}
		payload = append(payload, UpdateResult{
			Version:             "1.0", // TODO To be set to cfg.KruizePerformanceProfileVersion
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
