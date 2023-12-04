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

func Create_kruize_experiments(experiment_name string, cluster_identifier string, k8s_object []map[string]interface{}) ([]string, error) {
	// k8s_object (can) contain multiple containers of same k8s object type.
	data := map[string]string{
		"namespace":       kruizePayload.AssertAndConvertToString(k8s_object[0]["namespace"]),
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
	payload, err := kruizePayload.GetCreateExperimentPayload(experiment_name, cluster_identifier, containers, data)
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
			container_names, err := Create_kruize_experiments(experiment_name, cluster_identifier, k8s_object)
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

func Update_results(experiment_name string, payload_data []kruizePayload.UpdateResult) ([]kruizePayload.UpdateResult, error) {
	postBody, err := json.Marshal(payload_data)
	if err != nil {
		return nil, fmt.Errorf("unable to create payload: %v", err)
	}

	// Update metrics to kruize experiment
	url := cfg.KruizeUrl + "/updateResults"
	log.Debugf("\n Sending /updateResult request to kruize with payload - %s \n", string(postBody))
	res, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		kruizeAPIException.WithLabelValues("/updateResults").Inc()
		return nil, fmt.Errorf("an Error Occured while sending metrics: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	log.Debugf("\n Respose from API /updateResult - %s \n", string(body))
	if res.StatusCode != 201 {
		resdata := kruizePayload.UpdateResultResponse{}
		if err := json.Unmarshal(body, &resdata); err != nil {
			return nil, fmt.Errorf("can not unmarshal response data: %v", err)
		}

		// Comparing string should be changed once kruize fix it some standard error message
		if strings.Contains(resdata.Message, "because \"performanceProfile\" is null") {
			log.Error("Performance profile does not exist")
			log.Info("Tring to create resource_optimization_openshift performance profile")
			utils.Setup_kruize_performance_profile()
			if payload_data, err := Update_results(experiment_name, payload_data); err != nil {
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
	log.Debugf("\n Sending /updateRecommendations request to kruize - %s \n", q)
	res, err := client.Do(req)
	if err != nil {
		kruizeAPIException.WithLabelValues("/updateRecommendations").Inc()
		return nil, fmt.Errorf("error Occured while calling /updateRecommendations API %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	log.Debugf("\nResponse from /updateRecommendations - %s \n", string(body))
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

func Is_valid_recommendation(recommendation kruizePayload.Recommendation, experiment_name string, maxEndTime time.Time) bool {

	validRecommendationCode := "111000"
	_, recommendationIsValid := recommendation.Notifications[validRecommendationCode]
	if recommendationIsValid {
		// Convert the time object to the expected format
		formattedMaxEndTime := maxEndTime.UTC().Format("2006-01-02T15:04:05.000Z")
		recommendationData, timeStampisValid := recommendation.Data[formattedMaxEndTime]
		if !timeStampisValid {
			log.Error("recommendation not found for endtime: ", formattedMaxEndTime)
			invalidRecommendation.Inc()
			return false
		}
		LogKruizeErrors(recommendationData, formattedMaxEndTime, experiment_name)
		return true
	} else {
		return false
	}
}

func LogKruizeErrors(recommendationData kruizePayload.RecommendationData, formattedMaxEndTime string, experiment_name string) {

	// https://github.com/kruize/autotune/blob/master/design/NotificationCodes.md#detailed-codes
	errorNotificationCodes := map[string]string{
		"221001": "ERROR",
		"221002": "ERROR",
		"221003": "ERROR",
		"221004": "ERROR",
		"223001": "ERROR",
		"223002": "ERROR",
		"223003": "ERROR",
		"223004": "ERROR",
		"224001": "ERROR",
		"224002": "ERROR",
		"224003": "ERROR",
		"224004": "ERROR",
	}
	notificationSections := []map[string]kruizePayload.Notification{}

	// Timestamp level
	notificationsLevelTwo := recommendationData.Notifications
	if notificationsLevelTwo != nil {
		notificationSections = append(notificationSections, notificationsLevelTwo)
		// Term Level
		notificationsLevelThreeShortTerm := recommendationData.RecommendationTerms.Short_term.Notifications
		if notificationsLevelThreeShortTerm != nil {
			notificationSections = append(notificationSections, notificationsLevelThreeShortTerm)
			// Engine Level
			if recommendationData.RecommendationTerms.Short_term.RecommendationEngines != nil {
				shortTermCostNotification := recommendationData.RecommendationTerms.Short_term.RecommendationEngines.Cost.Notifications
				notificationSections = append(notificationSections, shortTermCostNotification)

				shortTermPerformanceNotification := recommendationData.RecommendationTerms.Short_term.RecommendationEngines.Performance.Notifications
				notificationSections = append(notificationSections, shortTermPerformanceNotification)
			}
		}
		notificationsLevelThreeMediumTerm := recommendationData.RecommendationTerms.Medium_term.Notifications
		if notificationsLevelThreeMediumTerm != nil {
			notificationSections = append(notificationSections, notificationsLevelThreeMediumTerm)
			if recommendationData.RecommendationTerms.Medium_term.RecommendationEngines != nil {
				mediumTermCostNotification := recommendationData.RecommendationTerms.Medium_term.RecommendationEngines.Cost.Notifications
				notificationSections = append(notificationSections, mediumTermCostNotification)

				mediumTermPerformanceNotification := recommendationData.RecommendationTerms.Medium_term.RecommendationEngines.Performance.Notifications
				notificationSections = append(notificationSections, mediumTermPerformanceNotification)
			}
		}
		notificationsLevelThreeLongTerm := recommendationData.RecommendationTerms.Long_term.Notifications
		if notificationsLevelThreeLongTerm != nil {
			notificationSections = append(notificationSections, notificationsLevelThreeLongTerm)
			if recommendationData.RecommendationTerms.Long_term.RecommendationEngines != nil {
				longTermCostNotification := recommendationData.RecommendationTerms.Long_term.RecommendationEngines.Cost.Notifications
				notificationSections = append(notificationSections, longTermCostNotification)

				longTermPerformanceNotification := recommendationData.RecommendationTerms.Long_term.RecommendationEngines.Performance.Notifications
				notificationSections = append(notificationSections, longTermPerformanceNotification)
			}
		}

	}

	for _, notificationBody := range notificationSections {
		for key := range notificationBody {
			_, keyExists := errorNotificationCodes[key]
			if keyExists {
				log.Error("kruize recommendation error; experiment_name: ", experiment_name, ", notification_code: ", key)
				kruizeRecommendationError.WithLabelValues(key).Inc()
			}

		}
	}

}
