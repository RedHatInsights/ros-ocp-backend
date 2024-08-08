package services

import (
	"encoding/json"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-gota/gota/dataframe"
	"github.com/go-playground/validator/v10"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	kafka_internal "github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	w "github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
)

var cfg *config.Config = config.GetConfig()

func ProcessReport(msg *kafka.Message, _ *kafka.Consumer) {
	log := logging.GetLogger()
	cfg = config.GetConfig()
	validate := validator.New()
	var kafkaMsg types.KafkaMsg
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

	log = logging.Set_request_details(kafkaMsg)

	// Create user account(if not present) for incoming archive.
	rh_account := model.RHAccount{
		Account: kafkaMsg.Metadata.Account,
		OrgId:   kafkaMsg.Metadata.Org_id,
	}
	if err := rh_account.CreateRHAccount(); err != nil {
		log.Errorf("unable to get or add record to rh_accounts table: %v. Error: %v", rh_account, err)
		return
	}

	// Create cluster record(if not present) for incoming archive.
	cluster := model.Cluster{
		TenantID:       rh_account.ID,
		SourceId:       kafkaMsg.Metadata.Source_id,
		ClusterUUID:    kafkaMsg.Metadata.Cluster_uuid,
		ClusterAlias:   kafkaMsg.Metadata.Cluster_alias,
		LastReportedAt: time.Now(),
	}
	if err := cluster.CreateCluster(); err != nil {
		log.Errorf("unable to get or add record to clusters table: %v. Error: %v", cluster, err)
		return
	}

	for _, file := range kafkaMsg.Files {
		data, err := utils.ReadCSVFromUrl(file)
		if err != nil {
			invalidCSV.Inc()
			log.Errorf("Unable to read CSV from URL. Error: %s", err)
			return
		}
		df := dataframe.LoadRecords(
			data,
			dataframe.WithTypes(types.CSVColumnMapping),
		)
		df, err = utils.Aggregate_data(df)
		if err != nil {
			log.Errorf("Error: %s", err)
			return
		}

		// grouping container(row in csv) by there deployement.
		k8s_object_groups := df.GroupBy("namespace", "k8s_object_type", "k8s_object_name").GetGroups()

		for _, v := range k8s_object_groups {

			all_interval_end_time := v.Col("interval_end").Records()
			maxEndTime, err := utils.MaxIntervalEndTime(all_interval_end_time)
			if err != nil {
				log.Errorf("unable to convert string to time: %s", err)
				continue
			}

			k8s_object := v.Maps()
			namespace := kruizePayload.AssertAndConvertToString(k8s_object[0]["namespace"])
			k8s_object_type := k8s_object[0]["k8s_object_type"].(string)
			k8s_object_name := k8s_object[0]["k8s_object_name"].(string)

			experiment_name := utils.GenerateExperimentName(
				kafkaMsg.Metadata.Org_id,
				kafkaMsg.Metadata.Source_id,
				kafkaMsg.Metadata.Cluster_uuid,
				namespace,
				k8s_object_type,
				k8s_object_name,
			)

			cluster_identifier := kafkaMsg.Metadata.Org_id + ";" + kafkaMsg.Metadata.Cluster_uuid
			container_names, err := kruize.Create_kruize_experiments(experiment_name, cluster_identifier, k8s_object)
			if err != nil {
				log.Error(err)
				continue
			}

			// Create workload entry into the table.
			workload := model.Workload{
				OrgId:           rh_account.OrgId,
				ClusterID:       cluster.ID,
				ExperimentName:  experiment_name,
				Namespace:       namespace,
				WorkloadType:    w.WorkloadType(k8s_object_type),
				WorkloadName:    k8s_object_name,
				Containers:      container_names,
				MetricsUploadAt: maxEndTime,
			}
			if err := workload.CreateWorkload(); err != nil {
				log.Errorf("unable to save workload record: %v. Error: %v", workload, err)
				continue
			}

			var k8s_object_chunks [][]kruizePayload.UpdateResult
			update_result_payload_data := kruizePayload.GetUpdateResultPayload(experiment_name, k8s_object)
			if len(update_result_payload_data) > cfg.KruizeMaxBulkChunkSize {
				k8s_object_chunks = sliceUpdatePayloadToChunks(update_result_payload_data)
			} else {
				k8s_object_chunks = append(k8s_object_chunks, update_result_payload_data)
			}

			for _, chunk := range k8s_object_chunks {
				usage_data_byte, err := kruize.Update_results(experiment_name, chunk)
				if err != nil {
					log.Error(err, experiment_name)
					continue
				}

				workload_metric_arr := []model.WorkloadMetrics{}
				for _, data := range usage_data_byte {

					interval_start_time, err := utils.ConvertISO8601StringToTime(data.Interval_start_time)
					if err != nil {
						log.Errorf("Error for start time: %s", err)
						continue
					}
					interval_end_time, err := utils.ConvertISO8601StringToTime(data.Interval_end_time)
					if err != nil {
						log.Errorf("Error for end time: %s", err)
						continue
					}

					for _, container := range data.Kubernetes_objects[0].Containers {
						container_usage_metrics, err := json.Marshal(container.Metrics)
						if err != nil {
							log.Errorf("Unable to marshal container usage data: %v", err)
							continue
						}

						workload_metric := model.WorkloadMetrics{
							OrgId:         rh_account.OrgId,
							WorkloadID:    workload.ID,
							ContainerName: container.Container_name,
							IntervalStart: interval_start_time,
							IntervalEnd:   interval_end_time,
							UsageMetrics:  container_usage_metrics,
						}
						workload_metric_arr = append(workload_metric_arr, workload_metric)
					}

				}
				if err := model.BatchInsertWorkloadMetrics(workload_metric_arr, rh_account.OrgId); err != nil {
					log.Errorf("unable to batch insert to workload_metrics table. Error: %v", err)
				}
			}
			// Sending recommendation request to recommendation-poller
			maxEndtimeFromReport := maxEndTime.UTC()
			messageData := types.RecommendationKafkaMsg{
				Request_id: kafkaMsg.Request_id,
				Metadata: types.RecommendationMetadata{
					Org_id:             kafkaMsg.Metadata.Org_id,
					Workload_id:        workload.ID,
					Max_endtime_report: maxEndtimeFromReport,
					Experiment_name:    experiment_name,
				},
			}

			msgBytes, err := json.Marshal(messageData)
			if err != nil {
				log.Error("Error marshaling JSON:", err)
				continue
			}

			msgProduceErr := kafka_internal.SendMessage(msgBytes, cfg.RecommendationTopic, experiment_name)
			if msgProduceErr != nil {
				log.Errorf("Failed to produce message: %v for experiment - %s and end_interval - %s\n", err, experiment_name, maxEndtimeFromReport)
			} else {
				log.Infof("Recommendation request sent for experiment - %s and end_interval - %s", experiment_name, maxEndtimeFromReport)
			}

		}
	}
}

func sliceUpdatePayloadToChunks(k8s_objects []kruizePayload.UpdateResult) [][]kruizePayload.UpdateResult {
	var chunks [][]kruizePayload.UpdateResult
	chunkSize := cfg.KruizeMaxBulkChunkSize
	for i := 0; i < len(k8s_objects); i += chunkSize {
		end := i + chunkSize

		if end > len(k8s_objects) {
			end = len(k8s_objects)
		}

		chunks = append(chunks, k8s_objects[i:end])
	}

	return chunks
}
