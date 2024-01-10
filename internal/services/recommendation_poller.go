package services

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/gommon/log"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
)

func commitKafkaMsg(msg *kafka.Message, consumer_object *kafka.Consumer) {
	_, err := consumer_object.CommitMessage(msg)
	if err != nil {
		log.Error("unable to commit msg: ", err)
	}
}

func requestAndSaveRecommendation(kafkaMsg types.RecommendationKafkaMsg, recommendationType string) bool {
	log := logging.GetLogger()
	experiment_name := kafkaMsg.Metadata.Experiment_name
	maxEndTimeFromReport := kafkaMsg.Metadata.Max_endtime_report
	poll_cycle_complete := false

	recommendation, err := kruize.Update_recommendations(experiment_name, maxEndTimeFromReport)
	if err != nil {
		end_interval := utils.ConvertDateToISO8601(maxEndTimeFromReport.String())
		if err.Error() == fmt.Sprintf("Recommendation for timestamp - \" %s \" does not exist", end_interval) {
			log.Infof("recommendation does not exist for timestamp - \" %s \"", end_interval)
		}
		log.Errorf("unable to list recommendation for: %v", err)
	}

	// TODO: Is_valid_recommendation to be called on every container record v20.1 upgrade on wards
	if kruize.Is_valid_recommendation(recommendation) {
		containers := recommendation[0].Kubernetes_objects[0].Containers
		for _, container := range containers {
			for _, v := range container.Recommendations.Data {
				marshalData, err := json.Marshal(v)
				if err != nil {
					log.Errorf("unable to list recommendation for: %v", err)
				}

				// Create RecommendationSet entry into the table.
				recommendationSet := model.RecommendationSet{
					WorkloadID:          kafkaMsg.Metadata.Workload_id,
					ContainerName:       container.Container_name,
					MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
					MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
					Recommendations:     marshalData,
				}
				if err := recommendationSet.CreateRecommendationSet(); err != nil {
					log.Errorf("unable to save a record into recommendation set: %v. Error: %v", recommendationSet, err)
				} else {
					log.Infof("%s - Recommendation saved for experiment - %s and end_interval - %s", recommendationType, experiment_name, recommendationSet.MonitoringEndTime)
					poll_cycle_complete = true
				}

				// Create entry into HistoricalRecommendationSet table.
				historicalRecommendationSet := model.HistoricalRecommendationSet{
					OrgId:               kafkaMsg.Metadata.Org_id,
					WorkloadID:          kafkaMsg.Metadata.Workload_id,
					ContainerName:       container.Container_name,
					MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
					MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
					Recommendations:     marshalData,
				}
				if err := historicalRecommendationSet.CreateHistoricalRecommendationSet(); err != nil {
					recommendationJSON, _ := json.Marshal(recommendation)
					log.Errorf("unable to get or add record to historical recommendation set table: %s. Error: %v", string(recommendationJSON), err)
					poll_cycle_complete = false
				}
			}
		}
	} else {
		poll_cycle_complete = true
		invalidRecommendation.Inc()
	}
	return poll_cycle_complete
}

func PollForRecommendations(msg *kafka.Message, consumer_object *kafka.Consumer) {
	log := logging.GetLogger()
	validate := validator.New()
	var kafkaMsg types.RecommendationKafkaMsg

	if !json.Valid([]byte(msg.Value)) {
		log.Errorf("received message on kafka topic is not vaild JSON: %s", msg.Value)
		commitKafkaMsg(msg, consumer_object)
		return
	}
	if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
		log.Errorf("unable to decode kafka message: %s", msg.Value)
		commitKafkaMsg(msg, consumer_object)
		return
	}
	if err := validate.Struct(kafkaMsg); err != nil {
		log.Errorf("invalid kafka message: %s", err)
		commitKafkaMsg(msg, consumer_object)
		return
	}
	log = logging.Set_request_details_recommendations(kafkaMsg)

	maxEndTimeFromReport := kafkaMsg.Metadata.Max_endtime_report
	workloadID := kafkaMsg.Metadata.Workload_id

	recommendation_stored_in_db, err := model.GetFirstRecommendationSetsByWorkloadID(workloadID)
	if err != nil {
		log.Errorf("Error while checking for recommendation_set record: %s", err)
	}
	recommendationFound := !reflect.ValueOf(recommendation_stored_in_db).IsZero()

	switch recommendationFound {
	case false:
		poll_cycle_complete := requestAndSaveRecommendation(kafkaMsg, "New")
		if poll_cycle_complete {
			commitKafkaMsg(msg, consumer_object)
		}
	case true:
		// MonitoringEndTime.UTC() defaults to 0001-01-01 00:00:00 +0000 UTC if not found
		if !recommendation_stored_in_db.MonitoringEndTime.UTC().IsZero() {
			duration := maxEndTimeFromReport.Sub(recommendation_stored_in_db.MonitoringEndTime.UTC())
			if int(duration.Hours()) >= cfg.RecommendationPollIntervalHours {
				poll_cycle_complete := requestAndSaveRecommendation(kafkaMsg, "Update")
				if poll_cycle_complete {
					commitKafkaMsg(msg, consumer_object)
				}
			} else {
				commitKafkaMsg(msg, consumer_object)
			}
		}
	}
}
