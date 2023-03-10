package services

import (
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logging.GetLogger()

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

	if err := processor.List_recommendations(kafkaMsg); err != nil {
		log.Errorf("Unable to list recommendation for: %v", err)
	}

}
