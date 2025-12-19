package services

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	namespacePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload/namespace"
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

func fetchRecommendationFromKruize(
	experimentName string,
	maxEndTime time.Time,
	experimentType types.PayloadType,
) (any, error) {
	log := logging.GetLogger()

	response, err := kruize.Update_recommendations(experimentName, maxEndTime, experimentType)
	if err != nil {
		endInterval := utils.ConvertDateToISO8601(maxEndTime.String())
		notFoundMsg := fmt.Sprintf("Recommendation for timestamp - \" %s \" does not exist", endInterval)

		if err.Error() == notFoundMsg {
			log.Errorf("unable to list recommendation for experiment : %s at interval: %v", experimentName, endInterval)
			if experimentType == types.PayloadTypeContainer {
				recommendationRequest.Inc()
			}
			if experimentType == types.PayloadTypeNamespace {
				namespaceRecommendationRequest.Inc()
			}
		}
		return nil, err
	}
	return response, nil
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

func transactionForNamespaceRecommendation(recommendationSetList []model.NamespaceRecommendationSet, histRecommendationSetList []model.HistoricalNamespaceRecommendationSet, experiment_name string, recommendationType string) error {
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
		if err := recommendationSet.CreateNamespaceRecommendationSet(tx); err != nil {
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

/* NOTE: Container and namespace paths are intentionally duplicated.
 * Unifying requires Go interfaces which adds complexity without clear benefit.
 * Adding interfaces will change the flow structurally, might increase the complexity of this service
 */

func requestAndSaveRecommendation(kafkaMsg types.RecommendationKafkaMsg, recommendationType string) bool {
	log := logging.GetLogger()
	cfg := config.GetConfig()
	experiment_name := kafkaMsg.Metadata.Experiment_name
	maxEndTimeFromReport := kafkaMsg.Metadata.Max_endtime_report
	poll_cycle_complete := false

	recommendationSetList := []model.RecommendationSet{}
	histRecommendationSetList := []model.HistoricalRecommendationSet{}

	namespaceRecommendationSetList := []model.NamespaceRecommendationSet{}
	namespaceHistRecommendationSetList := []model.HistoricalNamespaceRecommendationSet{}

	if kafkaMsg.Metadata.ExperimentType == types.PayloadTypeContainer {
		recommendationResponse, err := fetchRecommendationFromKruize(experiment_name, maxEndTimeFromReport, types.PayloadTypeContainer)
		if err != nil {
			return poll_cycle_complete
		}
		recommendationRequest.Inc()

		if recommendation, ok := recommendationResponse.([]kruizePayload.ListRecommendations); ok {

			if len(recommendation) == 0 || len(recommendation[0].Kubernetes_objects) == 0 {
				log.Warnf("empty recommendation response for experiment %s", experiment_name)
				return poll_cycle_complete
			}

			if recommendation[0].Experiment_type != string(types.PayloadTypeContainer) {
				log.Errorf("experiment type mismatch: expected %s, got %s", types.PayloadTypeContainer, recommendation[0].Experiment_type)
				return poll_cycle_complete
			}

			containers := recommendation[0].Kubernetes_objects[0].Containers
			for _, container := range containers {
				if kruize.Is_valid_recommendation(container.Recommendations, experiment_name, maxEndTimeFromReport) {
					for _, v := range container.Recommendations.Data {
						marshalData, err := json.Marshal(v)
						if err != nil {
							log.Errorf("unable to list recommendation for: %v", err)
							continue
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
		}

	}

	if !cfg.DisableNamespaceRecommendation {
		if kafkaMsg.Metadata.ExperimentType == types.PayloadTypeNamespace {
			namespaceRecommendation, err := fetchRecommendationFromKruize(experiment_name, maxEndTimeFromReport, types.PayloadTypeNamespace)
			if err != nil {
				return poll_cycle_complete
			}
			namespaceRecommendationRequest.Inc()

			if typedNamespaceObj, ok := namespaceRecommendation.(namespacePayload.NamespaceRecommendationResponse); ok {

				if len(typedNamespaceObj) == 0 || len(typedNamespaceObj[0].KubernetesObjects) == 0 {
					log.Warnf("empty namespace recommendation response for experiment %s", experiment_name)
					return poll_cycle_complete
				}

				if typedNamespaceObj[0].ExperimentType != string(types.PayloadTypeNamespace) {
					log.Errorf("experiment type mismatch: expected %s, got %s", types.PayloadTypeNamespace, typedNamespaceObj[0].ExperimentType)
					return poll_cycle_complete
				}

				typedNamespaceRecommendation := typedNamespaceObj[0].KubernetesObjects[0].Namespaces
				if kruize.Is_valid_recommendation(typedNamespaceRecommendation.Recommendations, experiment_name, maxEndTimeFromReport) {
					for _, v := range typedNamespaceRecommendation.Recommendations.Data {
						marshalData, err := json.Marshal(v)
						if err != nil {
							log.Errorf("unable to list recommendation for: %v", err)
							continue
						}
						recommendationSet := model.NamespaceRecommendationSet{
							WorkloadID:           kafkaMsg.Metadata.Workload_id,
							NamespaceName:        typedNamespaceRecommendation.Namespace,
							CPURequestCurrent:    v.Current.Requests.Cpu.Amount,
							MemoryRequestCurrent: v.Current.Requests.Memory.Amount,
							/* TODO
							 	* Add and populate columns for each term and recommendation type,
									cpu_variation_short_cost
									cpu_variation_short_performance
									cpu_variation_medium_cost
									cpu_variation_medium_performance
									cpu_variation_long_cost
									cpu_variation_long_performance
									memory_variation_short_cost
									memory_variation_short_performance
									memory_variation_medium_cost
									memory_variation_medium_performance
									memory_variation_long_cost
									memory_variation_long_performance
							*/
							MonitoringStartTime: v.RecommendationTerms.Short_term.MonitoringStartTime,
							MonitoringEndTime:   v.MonitoringEndTime,
							Recommendations:     marshalData,
						}
						namespaceRecommendationSetList = append(namespaceRecommendationSetList, recommendationSet)

						historicalRecommendationSet := model.HistoricalNamespaceRecommendationSet{
							OrgID:                kafkaMsg.Metadata.Org_id,
							WorkloadID:           kafkaMsg.Metadata.Workload_id,
							NamespaceName:        typedNamespaceRecommendation.Namespace,
							CPURequestCurrent:    v.Current.Requests.Cpu.Amount,
							MemoryRequestCurrent: v.Current.Requests.Memory.Amount,
							/* TODO
							 	* Add and populate columns for each term and recommendation type,
									cpu_variation_short_cost
									cpu_variation_short_performance
									cpu_variation_medium_cost
									cpu_variation_medium_performance
									cpu_variation_long_cost
									cpu_variation_long_performance
									memory_variation_short_cost
									memory_variation_short_performance
									memory_variation_medium_cost
									memory_variation_medium_performance
									memory_variation_long_cost
									memory_variation_long_performance
							*/
							MonitoringStartTime: v.RecommendationTerms.Short_term.MonitoringStartTime,
							MonitoringEndTime:   v.MonitoringEndTime,
							Recommendations:     marshalData,
						}
						namespaceHistRecommendationSetList = append(namespaceHistRecommendationSetList, historicalRecommendationSet)
					}
				} else {
					poll_cycle_complete = true
				}
			}
		}

	}

	if len(recommendationSetList) > 0 {
		txError := transactionForRecommendation(recommendationSetList, histRecommendationSetList, experiment_name, recommendationType)
		if txError == nil {
			poll_cycle_complete = true
			recommendationSuccess.Inc()
		} else {
			poll_cycle_complete = false
		}
	}

	if !cfg.DisableNamespaceRecommendation {
		if len(namespaceRecommendationSetList) > 0 {
			txError := transactionForNamespaceRecommendation(namespaceRecommendationSetList, namespaceHistRecommendationSetList, experiment_name, recommendationType)
			if txError == nil {
				poll_cycle_complete = true
				namespaceRecommendationSuccess.Inc()
			} else {
				poll_cycle_complete = false
			}
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

	var recommendation_stored_in_db any
	var checkRecommExistsErr error

	if kafkaMsg.Metadata.ExperimentType == types.PayloadTypeContainer {
		recommendation_stored_in_db, checkRecommExistsErr = model.GetFirstRecommendationSetsByWorkloadID(workloadID)
		if checkRecommExistsErr != nil {
			log.Errorf("error while checking for container recommendation_set record: %s", checkRecommExistsErr)
			return
		}
	} else if kafkaMsg.Metadata.ExperimentType == types.PayloadTypeNamespace && !cfg.DisableNamespaceRecommendation {
		recommendation_stored_in_db, checkRecommExistsErr = model.GetFirstNamespaceRecommendationSetsByWorkloadID(workloadID)
		if checkRecommExistsErr != nil {
			log.Errorf("error while checking for namespace recommendation_set record: %s", checkRecommExistsErr)
			return
		}
	} else {
		log.Errorf("unknown experiment type: %s", kafkaMsg.Metadata.ExperimentType)
		commitKafkaMsg(msg, consumer_object)
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
			var lastRecommRecordDate time.Time
			var lastRecommRecordID string

			switch v := recommendation_stored_in_db.(type) {
			case model.RecommendationSet:
				lastRecommRecordDate = v.MonitoringEndTime.UTC()
				lastRecommRecordID = v.ID
			case model.NamespaceRecommendationSet:
				lastRecommRecordDate = v.MonitoringEndTime.UTC()
				lastRecommRecordID = v.ID
			}
			if !lastRecommRecordDate.IsZero() {
				duration := maxEndTimeFromReport.Sub(lastRecommRecordDate)

				if int(duration.Hours()) >= cfg.RecommendationPollIntervalHours || utils.NeedRecommOnFirstOfMonth(lastRecommRecordDate, maxEndTimeFromReport) {
					poll_cycle_complete := requestAndSaveRecommendation(kafkaMsg, "Update")
					if poll_cycle_complete {
						commitKafkaMsg(msg, consumer_object)
					}
				} else {
					commitKafkaMsg(msg, consumer_object)
				}
			} else {
				commitKafkaMsg(msg, consumer_object)
				log.Warn("monitoring_end_time is set to 0001-01-01 00:00:00 +0000; recommendationID: ", lastRecommRecordID)
			}
			return
		}
	} else {
		commitKafkaMsg(msg, consumer_object)
	}

}
