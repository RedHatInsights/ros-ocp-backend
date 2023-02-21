package model

import (
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

type Recommendation struct {
	ID              uint   `gorm:"primaryKey;not null;autoIncrement"`
	ClusterID       string `gorm:"type:text"`
	Cluster         Cluster
	ExperimentName  string         `gorm:"type:text"`
	Namespace       string         `gorm:"type:text"`
	K8sObjectType   string         `gorm:"type:text"`
	K8sObjectName   string         `gorm:"type:text"`
	Containers      pq.StringArray `gorm:"type:text[];index:,type:gin"`
	Recommendations datatypes.JSON
}
