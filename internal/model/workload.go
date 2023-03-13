package model

import (
	"time"

	"gorm.io/gorm/clause"

	"github.com/lib/pq"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
)

type Workload struct {
	ID              uint `gorm:"primaryKey;not null;autoIncrement"`
	ClusterID       uint
	Cluster         Cluster
	ExperimentName  string                `gorm:"type:text"`
	Namespace       string                `gorm:"type:text"`
	WorkloadType    workload.WorkloadType `gorm:"type:text"`
	WorkloadName    string                `gorm:"type:text"`
	Containers      pq.StringArray        `gorm:"type:text[];index:,type:gin"`
	MetricsUploadAt time.Time
}

func (w *Workload) CreateWorkload() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cluster_id"}, {Name: "experiment_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"containers", "metrics_upload_at"}),
	}).Create(w)

	if result.Error != nil {
		return result.Error
	}

	return nil
}
