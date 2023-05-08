package kruize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logging.GetLogger()
var cfg *config.Config = config.GetConfig()
var experimentCreateAttempt bool = true

func Create_kruize_experiments(experiment_name string, k8s_object []map[string]interface{}) ([]string, error) {
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

	// Temporary fix
	// Currently, once Kruize pod inits it does not load performance-profile from DB
	if strings.Contains(resdata["message"].(string), "Performance Profile doesn't exist") && experimentCreateAttempt {
		log.Error("Performance profile does not exist")
		log.Info("Tring to create resource_optimization_openshift performance profile")
		utils.Setup_kruize_performance_profile()
		experimentCreateAttempt = false // Attempting only once
		container_names, err := Create_kruize_experiments(experiment_name, k8s_object)
		experimentCreateAttempt = true
		if err != nil {
			return nil, err
		} else {
			return container_names, nil
		}
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

func Update_results(experiment_name string, k8s_object []map[string]interface{}) ([]kruizePayload.UpdateResult, error) {
	data := map[string]string{
		"namespace":       k8s_object[0]["namespace"].(string),
		"k8s_object_type": k8s_object[0]["k8s_object_type"].(string),
		"k8s_object_name": k8s_object[0]["k8s_object_name"].(string),
		"interval_start":  utils.ConvertDateToISO8601(k8s_object[0]["interval_start"].(string)),
		"interval_end":    utils.ConvertDateToISO8601(k8s_object[0]["interval_end"].(string)),
	}
	payload_data := kruizePayload.GetUpdateResultPayload(experiment_name, k8s_object, data)
	postBody, err := json.Marshal(payload_data)
	if err != nil {
		return nil, fmt.Errorf("unable to create payload: %v", err)
	}

	// Update metrics to kruize experiment
	url := cfg.KruizeUrl + "/updateResults"

	res, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		return nil, fmt.Errorf("an Error Occured while sending metrics: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode == 201 {
		log.Info("Metrics uploaded successfully")
	} else {
		body, _ := io.ReadAll(res.Body)
		resdata := map[string]interface{}{}
		if err := json.Unmarshal(body, &resdata); err != nil {
			return nil, fmt.Errorf("can not unmarshal response data: %v", err)
		}
		if strings.Contains(resdata["message"].(string), "already contains result for timestamp") {
			log.Info(resdata["message"])
		}

		// Comparing string should be changed once kruize fix it some standard error message
		if strings.Contains(resdata["message"].(string), "because \"performanceProfile\" is null") {
			log.Error("Performance profile does not exist")
			log.Info("Tring to create resource_optimization_openshift performance profile")
			utils.Setup_kruize_performance_profile()
			if payload_data, err := Update_results(experiment_name, k8s_object); err != nil {
				return nil, err
			} else {
				return payload_data, nil
			}
		}

		if strings.Contains(resdata["message"].(string), fmt.Sprintf("Experiment name: %s not found", experiment_name)) {
			return nil, fmt.Errorf("%s", resdata["message"])
		}
	}

	return payload_data, nil
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
	q.Add("monitoring_end_time", utils.ConvertDateToISO8601(experiment.Monitoring_end_time))
	req.URL.RawQuery = q.Encode()
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error Occured while calling /listRecommendations API %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == 400 {
		data := map[string]interface{}{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unable to unmarshal response of /listRecommendations API %v", err)
		}
		return nil, fmt.Errorf(data["message"].(string))
	}
	response := []kruizePayload.ListRecommendations{}
	fmt.Println(string(body))
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response of /listRecommendations API %v", err)
	}

	return response, nil
}
