package processor

import (
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-gota/gota/dataframe"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

var log *logrus.Logger = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

func ProcessReport(msg *kafka.Message) {
	validate := validator.New()
	var kafkaMsg KafkaMsg
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

	for _, file := range kafkaMsg.Files {
		data, err := readCSVFromUrl(file)
		if err != nil {
			log.Errorf("Unable to read CSV from URL. Error: %s", err)
			return
		}
		df := dataframe.LoadRecords(data)
		create_kruize_experiments(df, kafkaMsg)
		list_of_experiments := update_results(df, kafkaMsg)
		for _, experiment := range list_of_experiments {
			if err := list_recommendations(experiment); err != nil {
				log.Errorf("Unable to list recommendation for: %v", list_of_experiments)
			}
		}
	}

}
