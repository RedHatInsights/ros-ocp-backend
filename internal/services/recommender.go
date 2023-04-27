package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	p "github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
)

func ProcessEvent(msg *kafka.Message) {
	log := logging.GetLogger()
	cfg := config.GetConfig()
	validate := validator.New()
	var kafkaMsg types.ExperimentEvent
	if !json.Valid([]byte(msg.Value)) {
		log.Errorf("Received message on kafka topic is not vaild JSON: %s", msg.Value)
		return
	}
	if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
		log.Errorf("Unable to decode kafka message: %s", msg.Value)
		return
	}
	if err := validate.Struct(kafkaMsg); err != nil {
		log.Errorf("Invalid kafka message: %s", err)
		return
	}

	currentTime := time.Now().UTC()
	if currentTime.Before(kafkaMsg.Fetch_time) {
		t := kafkaMsg.Fetch_time.Sub(currentTime)
		log.Info("Sleeping for: ", t)
		time.Sleep(t)
	}
	data, err := kruize.List_recommendations(kafkaMsg)
	if err != nil {
		if err.Error() == fmt.Sprintf("Recommendation for timestamp - \" %s \" does not exist", utils.ConvertDateToISO8601(kafkaMsg.Monitoring_end_time)) {
			return
		}
		log.Errorf("Unable to list recommendation for: %v", err)
		return
	}

	if is_valid_recommendation(data) {
		containers := data[0].Kubernetes_objects[0].Containers
		for _, container := range containers {
			for _, v := range container.Recommendations.Data {
				if len(v.Duration_based.Short_term.Notifications) == 0 {
					marshalData, err := json.Marshal(v)
					if err != nil {
						log.Errorf("Unable to list recommendation for: %v", err)
					}

					// Create RecommendationSet entry into the table.
					recommendationSet := model.RecommendationSet{
						WorkloadID:          kafkaMsg.WorkloadID,
						ContainerName:       container.Container_name,
						MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
						MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
						Recommendations:     marshalData,
					}
					if err := recommendationSet.CreateRecommendationSet(); err != nil {
						log.Errorf("unable to get or add record to recommendation set table: %v. Error: %v", recommendationSet, err)
						return
					}
				}
			}
		}
	} else {
		if kafkaMsg.Attempt > 5 {
			return
		}
		kafkaMsg.Attempt = kafkaMsg.Attempt + 1
		if _, err := kruize.Update_results(kafkaMsg.Experiment_name, kafkaMsg.K8s_object); err != nil {
			log.Error(err)
		}
		kafkaMsg.Fetch_time = time.Now().UTC().Add(time.Minute * time.Duration(2))

		msgBytes, err := json.Marshal(kafkaMsg)
		if err != nil {
			log.Errorf("Unable convert list_of_experiments to json: %s", err)
		}
		p.SendMessage(msgBytes, &cfg.ExperimentsTopic)
	}

}

func is_valid_recommendation(d []kruizePayload.ListRecommendations) bool {
	if len(d) > 0 {
		notifications := d[0].Kubernetes_objects[0].Containers[0].Recommendations.Notifications
		if len(notifications) > 0 && notifications[0].Message == "Duration Based Recommendations Available" {
			return true
		} else {
			return false
		}
	}
	return false
}
