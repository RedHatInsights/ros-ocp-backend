package types

import "time"

type ExperimentEvent struct {
	WorkloadID          uint                     `validate:"required"`
	Experiment_name     string                   `validate:"required"`
	K8s_object_name     string                   `validate:"required"`
	K8s_object_type     string                   `validate:"required"`
	Namespace           string                   `validate:"required"`
	Fetch_time          time.Time                `validate:"required"`
	Monitoring_end_time string                   `validate:"required"`
	Attempt             int                      `validate:"required"`
	K8s_object          []map[string]interface{} `validate:"required"`
	Kafka_request_msg   KafkaMsg
}
