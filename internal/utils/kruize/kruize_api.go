package kruize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry = logging.GetLogger()
var cfg *config.Config = config.GetConfig()
var experimentCreateAttempt bool = true

func Create_kruize_experiments(experiment_name string, k8s_object []map[string]interface{}) ([]string, error) {
	// k8s_object (can) contain multiple containers of same k8s object type.
	data := map[string]string{
		"namespace":       k8s_object[0]["namespace"].(string),
		"k8s_object_type": k8s_object[0]["k8s_object_type"].(string),
		"k8s_object_name": k8s_object[0]["k8s_object_name"].(string),
	}
	unique_containers := []string{}
	containers := []map[string]string{}
	for _, row := range k8s_object {
		container := row["container_name"].(string)
		if !utils.StringInSlice(container, unique_containers) {
			unique_containers = append(unique_containers, container)
			containers = append(containers, map[string]string{
				"container_name":       container,
				"container_image_name": row["image_name"].(string),
			})
		}
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
		kruizeAPIException.WithLabelValues("/createExperiment").Inc()
		return nil, fmt.Errorf("error Occured while creating experiment: %v", err)
	}
	if res.StatusCode != 201 {
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

		if strings.Contains(resdata["message"].(string), "Experiment name already exists") {
			log.Debug("Experiment already exist")
		} else {
			return nil, fmt.Errorf("%s", resdata["message"])
		}
	}

	container_names := make([]string, 0, len(containers))
	for _, value := range containers {
		container_names = append(container_names, value["container_name"])
	}

	return container_names, nil
}

func Update_results(experiment_name string, k8s_object []map[string]interface{}) ([]kruizePayload.UpdateResult, error) {
	payload_data := kruizePayload.GetUpdateResultPayload(experiment_name, k8s_object)
	postBody, err := json.Marshal(payload_data)
	if err != nil {
		return nil, fmt.Errorf("unable to create payload: %v", err)
	}

	// Update metrics to kruize experiment
	url := cfg.KruizeUrl + "/updateResults"

	res, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		kruizeAPIException.WithLabelValues("/updateResults").Inc()
		return nil, fmt.Errorf("an Error Occured while sending metrics: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		body, _ := io.ReadAll(res.Body)
		resdata := kruizePayload.UpdateResultResponse{}
		if err := json.Unmarshal(body, &resdata); err != nil {
			return nil, fmt.Errorf("can not unmarshal response data: %v", err)
		}

		// Comparing string should be changed once kruize fix it some standard error message
		if strings.Contains(resdata.Message, "because \"performanceProfile\" is null") {
			log.Error("Performance profile does not exist")
			log.Info("Tring to create resource_optimization_openshift performance profile")
			utils.Setup_kruize_performance_profile()
			if payload_data, err := Update_results(experiment_name, k8s_object); err != nil {
				return nil, err
			} else {
				return payload_data, nil
			}
		}

		if len(resdata.Data) > 0 {
			for _, err := range resdata.Data {
				if err.Errors[0].Message == "An entry for this record already exists!" {
					continue
				} else {
					log.Error(err.Errors[0].Message)
				}
			}
		}
	}

	return payload_data, nil
}

func Update_recommendations(experiment_name string, interval_end_time time.Time) ([]kruizePayload.ListRecommendations, error) {
	url := cfg.KruizeUrl + "/updateRecommendations"
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("an Error Occured %v", err)
	}
	q := req.URL.Query()
	q.Add("experiment_name", experiment_name)
	q.Add("interval_end_time", utils.ConvertDateToISO8601(interval_end_time.String()))
	req.URL.RawQuery = q.Encode()
	res, err := client.Do(req)
	if err != nil {
		kruizeAPIException.WithLabelValues("/updateRecommendations").Inc()
		return nil, fmt.Errorf("error Occured while calling /updateRecommendations API %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == 400 {
		data := map[string]interface{}{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unable to unmarshal response of /updateRecommendations API %v", err)
		}
		return nil, fmt.Errorf(data["message"].(string))
	}
	response := []kruizePayload.ListRecommendations{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response of /updateRecommendations API %v", err)
	}

	return response, nil

}

func Is_valid_recommendation(d []kruizePayload.ListRecommendations, experiment_name string) bool {
	if len(d) > 0 {

		// To maintain a local reference the following map has been created from 
		// https://github.com/kruize/autotune/blob/master/design/NotificationCodes.md#detailed-codes
		notificationCodeValidities := map[string]bool{
			"112101": true,
			"120001": false,
			"221001": false,
			"221002": false,
			"221003": false,
			"221004": false,
			"223001": false,
			"223002": false,
			"223003": false,
			"223004": false,
			"224001": false,
			"224002": false,
			"224003": false,
			"224004": false,
		}

		notifications := d[0].Kubernetes_objects[0].Containers[0].Recommendations.Notifications

		for key := range notifications{
			isValid, keyExists := notificationCodeValidities[key]
			if !keyExists {
				return false
			} 

			if !isValid {
				// Setting the metric counter to 1 as we expect a single metric
				// for a combination of notification_code and experiment_name
				kruizeInvalidRecommendation.WithLabelValues(key, experiment_name).Set(1)
				return false
			} else {
				return true
			}
			

		}
	}
	return false
}
