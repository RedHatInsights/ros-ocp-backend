package model

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

type RecommendationSet struct {
	ClusterID       uint
	Cluster         Cluster
	ExperimentName  string         `gorm:"type:text"`
	Namespace       string         `gorm:"type:text"`
	WorkloadType    string         `gorm:"type:text"`
	WorkloadName    string         `gorm:"type:text"`
	Containers      pq.StringArray `gorm:"type:text[];index:,type:gin"`
	Recommendations datatypes.JSON
	CreatedAt       time.Time
}
