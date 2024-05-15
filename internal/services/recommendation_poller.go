package services

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
)

func commitKafkaMsg(msg *kafka.Message, consumer_object *kafka.Consumer) {
	log := logging.GetLogger()
	_, err := consumer_object.CommitMessage(msg)
	if err != nil {
		log.Error("unable to commit msg: ", err)
	}
}

func transactionForRecommendation(recommendationSetList []model.RecommendationSet, histRecommendationSetList []model.HistoricalRecommendationSet, experiment_name string, recommendationType string) error {
	log := logging.GetLogger()
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	for _, recommendationSet := range recommendationSetList {
		if err := recommendationSet.CreateRecommendationSet(tx); err != nil {
			log.Errorf("unable to save a record into recommendation set: %v. Error: %v", recommendationSet, err)
			tx.Rollback()
			return err
		} else {
			log.Infof("%s - Recommendation saved for experiment - %s and end_interval - %s", recommendationType, experiment_name, recommendationSet.MonitoringEndTimeStr)
		}
	}
	for _, historicalRecommendationSet := range histRecommendationSetList {
		if err := historicalRecommendationSet.CreateHistoricalRecommendationSet(tx); err != nil {
			log.Errorf("unable to get or add record to historical recommendation set table: %v. Error: %v", historicalRecommendationSet, err)
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
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
		return poll_cycle_complete
	}

	containers := recommendation[0].Kubernetes_objects[0].Containers
	recommendationSetList := []model.RecommendationSet{}
	histRecommendationSetList := []model.HistoricalRecommendationSet{}

	for _, container := range containers {
		if kruize.Is_valid_recommendation(container.Recommendations, experiment_name, maxEndTimeFromReport) {
			for _, v := range container.Recommendations.Data {
				marshalData, err := json.Marshal(v)
				if err != nil {
					log.Errorf("unable to list recommendation for: %v", err)
				}
				// Create RecommendationSet entry into the table.
				recommendationSet := model.RecommendationSet{
					WorkloadID:          kafkaMsg.Metadata.Workload_id,
					ContainerName:       container.Container_name,
					MonitoringStartTime: v.RecommendationTerms.Short_term.MonitoringStartTime,
					MonitoringEndTime:   v.MonitoringEndTime,
					Recommendations:     marshalData,
				}
				recommendationSetList = append(recommendationSetList, recommendationSet)

				// Create entry into HistoricalRecommendationSet table.
				historicalRecommendationSet := model.HistoricalRecommendationSet{
					OrgId:               kafkaMsg.Metadata.Org_id,
					WorkloadID:          kafkaMsg.Metadata.Workload_id,
					ContainerName:       container.Container_name,
					MonitoringStartTime: v.RecommendationTerms.Short_term.MonitoringStartTime,
					MonitoringEndTime:   v.MonitoringEndTime,
					Recommendations:     marshalData,
				}
				histRecommendationSetList = append(histRecommendationSetList, historicalRecommendationSet)
			}
		} else {
			poll_cycle_complete = true
			continue
		}
	}
	if len(recommendationSetList) > 0 {
		txError := transactionForRecommendation(recommendationSetList, histRecommendationSetList, experiment_name, recommendationType)
		if txError == nil {
			poll_cycle_complete = true
		} else {
			poll_cycle_complete = false
		}
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
		return
	}
	workloadExists := model.WorkloadExistsByID(workloadID)

	if workloadExists { // Housekeeper may wipe workload record by the time poller requests for a recommendation
		recommendationFound := !reflect.ValueOf(recommendation_stored_in_db).IsZero()

		switch recommendationFound {
		case false:
			poll_cycle_complete := requestAndSaveRecommendation(kafkaMsg, "New")
			if poll_cycle_complete {
				commitKafkaMsg(msg, consumer_object)
			}
			// To consume upcoming Kafka msg, explicitly
			return
		case true:
			// MonitoringEndTime.UTC() defaults to 0001-01-01 00:00:00 +0000 UTC if not set
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
			return
		}
	}

}
