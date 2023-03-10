package processor

import (
	"encoding/json"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-gota/gota/dataframe"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	p "github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
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
		df = Aggregate_data(df)

		// grouping container(row in csv) by there deployement.
		k8s_object_groups := df.GroupBy("namespace", "k8s_object_type", "k8s_object_name").GetGroups()

		// looping over each group.
		for _, k8s_object_group := range k8s_object_groups {
			k8s_object := k8s_object_group.Maps()

			experiment_name := generateExperimentName(
				kafkaMsg.Metadata.Org_id,
				kafkaMsg.Metadata.Cluster_id,
				k8s_object[0]["namespace"].(string),
				k8s_object[0]["k8s_object_type"].(string),
				k8s_object[0]["k8s_object_name"].(string),
			)

			if err := create_kruize_experiments(experiment_name, k8s_object); err != nil {
				log.Error(err)
				continue
			}
			if err := update_results(experiment_name, k8s_object); err != nil {
				log.Error(err)
				continue
			}

			// Sending list_of_experiments to rosocp.kruize.experiments topic.
			experimentEventMsg := types.ExperimentEvent{
				Experiment_name: experiment_name,
				K8s_object_name: k8s_object[0]["k8s_object_name"].(string),
				K8s_object_type: k8s_object[0]["k8s_object_type"].(string),
				Namespace:       k8s_object[0]["namespace"].(string),
				Fetch_time:      time.Now().Add(time.Minute * time.Duration(2)),
			}

			msgBytes, err := json.Marshal(experimentEventMsg)
			if err != nil {
				log.Errorf("Unable convert list_of_experiments to json: %s", err)
			}
			p.SendMessage(msgBytes, &cfg.ExperimentsTopic)

			// for _, experiment := range list_of_experiments {
			// 	if err := list_recommendations(experiment); err != nil {
			// 		log.Errorf("Unable to list recommendation for: %v Error: %v", list_of_experiments, err)
			// 	}
			// }
		}

	}

}
