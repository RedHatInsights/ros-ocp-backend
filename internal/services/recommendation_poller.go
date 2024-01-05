package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
)

func FetchRecommendations(msg *kafka.Message, consumer_object *kafka.Consumer) {
	log := logging.GetLogger()
	validate := validator.New()
	var kafkaMsg types.RecommendationKafkaMsg

	if !json.Valid([]byte(msg.Value)) {
		log.Errorf("received message on kafka topic is not vaild JSON: %s", msg.Value)
		return
	}
	if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
		log.Errorf("unable to decode kafka message: %s", msg.Value)
		return
	}
	if err := validate.Struct(kafkaMsg); err != nil {
		log.Errorf("invalid kafka message: %s", err)
		return
	}
	log = logging.Set_request_details_recommendations(kafkaMsg)

	var poll_cycle_complete bool = false
	var poll_for_recommendation bool = false

	var experiment_name string = kafkaMsg.Metadata.Experiment_name
	var maxEndTimeFromReport time.Time = kafkaMsg.Metadata.Max_endtime_report

	var workloadID uint = kafkaMsg.Metadata.Workload_id
	var orgID string = kafkaMsg.Metadata.Org_id
	var isNewRecord bool = kafkaMsg.Metadata.New_record

	if isNewRecord {
		poll_for_recommendation = true
	} else {
		recommendation_stored_in_db, err := model.GetFirstRecommendationSetsByWorkloadID(workloadID)
		if err != nil {
			log.Errorf("Error while checking for recommendation_set record: %s", err)
		}
		duration := maxEndTimeFromReport.Sub(recommendation_stored_in_db.MonitoringEndTime.UTC())
		if int(duration.Hours()) >= cfg.RecommendationFetchDelay {
			poll_for_recommendation = true
		}
	}

	if poll_for_recommendation {
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
						WorkloadID:          workloadID,
						ContainerName:       container.Container_name,
						MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
						MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
						Recommendations:     marshalData,
					}
					if err := recommendationSet.CreateRecommendationSet(); err != nil {
						log.Errorf("unable to save a record into recommendation set: %v. Error: %v", recommendationSet, err)
						continue
					} else {
						log.Infof("Recommendation saved for experiment - %s and end_interval - %s", experiment_name, recommendationSet.MonitoringEndTime)
					}

					// Create entry into HistoricalRecommendationSet table.
					historicalRecommendationSet := model.HistoricalRecommendationSet{
						OrgId:               orgID,
						WorkloadID:          workloadID,
						ContainerName:       container.Container_name,
						MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
						MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
						Recommendations:     marshalData,
					}
					if err := historicalRecommendationSet.CreateHistoricalRecommendationSet(); err != nil {
						recommendationJson, _ := json.Marshal(recommendation)
						log.Errorf("unable to get or add record to historical recommendation set table: %s. Error: %v", string(recommendationJson), err)
						continue
					}
					poll_cycle_complete = true
				}
			}
		} else {
			poll_cycle_complete = true
			// TODO: Needs to be removed Kruize 20.1 upgrade on wards
			invalidRecommendation.Inc()
		}
	}

	if poll_cycle_complete {
		_, err := consumer_object.CommitMessage(msg)
		if err != nil {
			log.Errorf("error committing offset: %v\n", err)
		}
	}

}
