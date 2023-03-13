package services

import (
	"encoding/json"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	p "github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

func ProcessEvent(msg *kafka.Message) {
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

	currentTime := time.Now()
	if currentTime.Before(kafkaMsg.Fetch_time) {
		t := kafkaMsg.Fetch_time.Sub(currentTime)
		log.Info("Sleeping for: ", t)
		time.Sleep(t)
	}
	data, err := processor.List_recommendations(kafkaMsg)
	if err != nil {
		log.Errorf("Unable to list recommendation for: %v", err)
	}

	if is_valid_recommendation(data) {
		for _, v := range data[0].Kubernetes_objects[0].Containers[0].Recommendations {
			marshalData, err := json.Marshal(v)
			if err != nil {
				log.Errorf("Unable to list recommendation for: %v", err)
			}
			// Create RecommendationSet entry into the table.
			recommendationSet := model.RecommendationSet{
				WorkloadID:      kafkaMsg.WorkloadID,
				Recommendations: marshalData,
			}
			if err := recommendationSet.CreateRecommendationSet(); err != nil {
				log.Errorf("unable to get or add record to recommendation set table: %v. Error: %v", recommendationSet, err)
				return
			}
		}

	} else if kafkaMsg.Fetch_attempt < 3 {
		kafkaMsg.Fetch_time = time.Now().Add(time.Minute * time.Duration(2))
		kafkaMsg.Fetch_attempt = kafkaMsg.Fetch_attempt + 1

		msgBytes, err := json.Marshal(kafkaMsg)
		if err != nil {
			log.Errorf("Unable convert list_of_experiments to json: %s", err)
		}
		p.SendMessage(msgBytes, &cfg.ExperimentsTopic)
	}

}

func is_valid_recommendation(data []kruizePayload.ListRecommendations) bool {
	for _, v := range data[0].Kubernetes_objects[0].Containers[0].Recommendations {
		asd := v.Duration_based.Short_term.Config
		if asd != (kruizePayload.ConfigObject{}) {
			return true
		}
	}
	return false
}
