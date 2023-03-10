package types

import "time"

type ExperimentEvent struct {
	Experiment_name string    `validate:"required"`
	K8s_object_name string    `validate:"required"`
	K8s_object_type string    `validate:"required"`
	Namespace       string    `validate:"required"`
	Fetch_time      time.Time `validate:"required"`
}
