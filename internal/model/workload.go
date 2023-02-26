package model

import (
	"time"

	"github.com/lib/pq"
)

type Workload struct {
	ID              uint `gorm:"primaryKey;not null;autoIncrement"`
	ClusterID       uint
	Cluster         Cluster
	ExperimentName  string         `gorm:"type:text"`
	Namespace       string         `gorm:"type:text"`
	WorkloadType    string         `gorm:"type:text"`
	WorkloadName    string         `gorm:"type:text"`
	Containers      pq.StringArray `gorm:"type:text[];index:,type:gin"`
	MetricsUploadAt time.Time
}
