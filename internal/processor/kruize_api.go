package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
)

func create_kruize_experiments(experiment_name string, k8s_object []map[string]interface{}) ([]string, error) {
	// k8s_object (can) contain multiple containers of same k8s object type.
	data := map[string]string{
		"namespace":       k8s_object[0]["namespace"].(string),
		"k8s_object_type": k8s_object[0]["k8s_object_type"].(string),
		"k8s_object_name": k8s_object[0]["k8s_object_name"].(string),
	}
	containers := []map[string]string{}
	for _, row := range k8s_object {
		containers = append(containers, map[string]string{
			"container_name":       row["container_name"].(string),
			"container_image_name": row["image_name"].(string),
		})
	}
	payload, err := kruizePayload.GetCreateExperimentPayload(experiment_name, containers, data)
	if err != nil {
		return nil, fmt.Errorf("unable to create payload: %v", err)
	}
	// Create experiment in kruize
	url := cfg.KruizeUrl + "/createExperiment"
	if err != nil {
		return nil, fmt.Errorf("unable to marshal payload to json: %v", err)

	}
	res, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("error Occured while creating experiment: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	resdata := map[string]interface{}{}
	if err := json.Unmarshal(body, &resdata); err != nil {
		return nil, fmt.Errorf("can not unmarshal response data: %v", err)
	}
	if strings.Contains(resdata["message"].(string), "is duplicate") {
		log.Info("Experiment already exist")
	}
	if res.StatusCode == 201 {
		log.Info("Experiment Created successfully")
	}

	container_names := make([]string, 0, len(containers))
	for _, value := range containers {
		container_names = append(container_names, value["container_name"])
	}

	return container_names, nil
}

func Update_results(experiment_name string, k8s_object []map[string]interface{}) error {
	data := map[string]string{
		"namespace":       k8s_object[0]["namespace"].(string),
		"k8s_object_type": k8s_object[0]["k8s_object_type"].(string),
		"k8s_object_name": k8s_object[0]["k8s_object_name"].(string),
		"interval_start":  convertDateToISO8601(k8s_object[0]["interval_start"].(string)),
		"interval_end":    convertDateToISO8601(k8s_object[0]["interval_end"].(string)),
	}
	payload_data, err := kruizePayload.GetUpdateResultPayload(experiment_name, k8s_object, data)
	if err != nil {
		return fmt.Errorf("unable to create payload: %v", err)
	}

	// Update metrics to kruize experiment
	url := cfg.KruizeUrl + "/updateResults"

	res, err := http.Post(url, "application/json", bytes.NewBuffer(payload_data))
	if err != nil {
		return fmt.Errorf("an Error Occured while sending metrics: %v", err)
	}
	if res.StatusCode == 201 {
		log.Info("Metrics uploaded successfully")
	} else {
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		resdata := map[string]interface{}{}
		if err := json.Unmarshal(body, &resdata); err != nil {
			return fmt.Errorf("can not unmarshal response data: %v", err)
		}
		if strings.Contains(resdata["message"].(string), "already contains result for timestamp") {
			log.Info(resdata["message"])
		}
	}

	return nil
}

func List_recommendations(experiment types.ExperimentEvent) ([]kruizePayload.ListRecommendations, error) {
	url := cfg.KruizeUrl + "/listRecommendations"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("an Error Occured %v", err)
	}
	q := req.URL.Query()
	q.Add("experiment_name", experiment.Experiment_name)
	q.Add("monitoring_end_time", experiment.Monitoring_end_time)
	req.URL.RawQuery = q.Encode()
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error Occured while calling /listRecommendations API %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	response := []kruizePayload.ListRecommendations{}
	fmt.Println(string(body))
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response of /listRecommendations API %v", err)
	}

	return response, nil
}
