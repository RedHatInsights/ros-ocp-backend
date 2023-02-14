package processor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"encoding/json"

	"github.com/go-gota/gota/dataframe"
)

func create_kruize_experiments(df dataframe.DataFrame, kafkaMsg KafkaMsg) {
	payload_data := map[string]interface{}{
		"performanceProfile":      "resource-optimization-openshift",
		"mode":                    "monitor",
		"targetCluster":           "remote",
		"trial_settings":          map[string]string{"measurement_duration": "15min"},
		"recommendation_settings": map[string]string{"threshold": "0.1"},
	}
	namspaces := get_all_namespaces(df)
	for _, namespace := range namspaces {
		deployments := get_all_deployments_from_namespace(df, namespace)
		for _, deployment := range deployments {
			containers := get_all_containers_and_images_from_deployment(df, namespace, deployment)
			data := []map[string]string{}
			for _, container := range containers {
				c := map[string]string{
					"container_name": container["container_name"].(string),
					"image":          container["image_name"].(string),
				}
				data = append(data, c)
			}
			payload_data["containers"] = data
			payload_data["experiment_name"] = fmt.Sprintf("%s|%s|%s|%s", kafkaMsg.Metadata.Org_id, kafkaMsg.Metadata.Cluster_id, namespace, deployment)
			payload_data["deployment_name"] = deployment
			payload_data["namespace"] = namespace
			wrapper := []map[string]interface{}{
				payload_data,
			}

			// Create experiment in kruize
			url := cfg.KruizeUrl + "/createExperiment"
			postBody, err := json.Marshal(wrapper)
			if err != nil {
				log.Errorf("unable to marshal payload to json: %v", err)
				continue
			}
			res, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
			if err != nil {
				log.Errorf("An Error Occured while creating experiment: %v", err)
				continue
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			resdata := map[string]interface{}{}
			if err := json.Unmarshal(body, &resdata); err != nil {
				log.Errorf("can not unmarshal response data: %v", err)
				continue
			}
			if strings.Contains(resdata["message"].(string), "is duplicate") {
				log.Info("Experiment already exist")
			}
			if res.StatusCode == 201 {
				log.Info("Experiment Created successfully")
			}

		}
	}
}

func update_results(df dataframe.DataFrame, kafkaMsg KafkaMsg) []map[string]string {
	list_of_experiments := []map[string]string{}
	payload_data := map[string]interface{}{}
	namspaces := get_all_namespaces(df)
	for _, namespace := range namspaces {
		deployments := get_all_deployments_from_namespace(df, namespace)
		for _, deployment := range deployments {
			containers_with_metrics := get_all_containers_and_metrics(df, namespace, deployment)
			all_containers := []map[string]interface{}{}
			for _, container := range containers_with_metrics {
				container_data := make_container_data(container)
				all_containers = append(all_containers, container_data)
			}

			experiment_name := fmt.Sprintf("%s|%s|%s|%s", kafkaMsg.Metadata.Org_id, kafkaMsg.Metadata.Cluster_id, namespace, deployment)

			payload_data["experiment_name"] = experiment_name
			// below timestamp variable needs to be revisited once timestamp location in payload is confirmed
			payload_data["start_timestamp"] = convertDateToISO8601("containers_with_metrics[0][\"interval_start\"]")
			payload_data["end_timestamp"] = convertDateToISO8601("containers_with_metrics[0][\"interval_end\"]")
			payload_data["deployments"] = []map[string]interface{}{
				{
					"containers":      all_containers,
					"deployment_name": deployment,
					"namespace":       namespace,
					"pod_metrics":     []string{},
				},
			}

			list_of_experiments = append(list_of_experiments, map[string]string{
				"experiment_name": experiment_name,
				"deployment_name": deployment,
				"namespace":       namespace,
			})
			wrapper := []map[string]interface{}{
				payload_data,
			}

			// Update metrics to kruize experiment
			url := cfg.KruizeUrl + "/updateResults"
			postBody, err := json.Marshal(wrapper)
			if err != nil {
				log.Errorf("unable to marshal payload to json: %v", err)
				continue
			}
			res, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
			if err != nil {
				log.Errorf("An Error Occured while sending metrics: %v", err)
				continue
			}
			if res.StatusCode == 201 {
				log.Info("Metrics uploaded successfully")
			} else {
				defer res.Body.Close()
				body, _ := io.ReadAll(res.Body)
				resdata := map[string]interface{}{}
				if err := json.Unmarshal(body, &resdata); err != nil {
					log.Errorf("can not unmarshal response data: %v", err)
					continue
				}
				if strings.Contains(resdata["message"].(string), "already contains result for timestamp") {
					log.Info(resdata["message"])
				}
			}
		}
	}
	return list_of_experiments
}

func list_recommendations(experiment map[string]string) error {
	error_string := "Failed while listing recommendations from kruize"
	params := map[string]string{
		"experiment_name": experiment["experiment_name"],
		"deployment_name": experiment["deployment_name"],
		"namespace":       experiment["namespace"],
	}

	// list recommendation from kruize
	url := cfg.KruizeUrl + "/listRecommendations"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("An Error Occured %v", err)
		return errors.New(error_string)
	}
	q := req.URL.Query()
	q.Add("experiment_name", params["experiment_name"])
	q.Add("deployment_name", params["deployment_name"])
	q.Add("namespace", params["namespace"])
	res, err := client.Do(req)
	if err != nil {
		log.Errorf("Error Occured while calling /listRecommendations API %v", err)
		return errors.New(error_string)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	fmt.Println(string(body))
	return nil
}
