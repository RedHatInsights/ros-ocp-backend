package model

import (
	"time"

	"gorm.io/gorm/clause"

	"github.com/lib/pq"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
	"gorm.io/gorm"
)

type Workload struct {
	ID              uint   `gorm:"primaryKey;not null;autoIncrement"`
	OrgId           string `gorm:"type:text;not null"`
	ClusterID       uint
	Cluster         Cluster               `gorm:"foreignKey:ClusterID" json:"-"`
	ExperimentName  string                `gorm:"type:text"`
	Namespace       string                `gorm:"type:text"`
	WorkloadType    workload.WorkloadType `gorm:"type:text"`
	WorkloadName    string                `gorm:"type:text"`
	Containers      pq.StringArray        `gorm:"type:text[];index:,type:gin"`
	MetricsUploadAt time.Time
	WorkloadTypeStr string `gorm:"-"`
}

func (w *Workload) AfterFind(tx *gorm.DB) error {
	if w.WorkloadType != "" {
		w.WorkloadTypeStr = string(w.WorkloadType)
	}
	return nil
}

func (w *Workload) CreateWorkload() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "cluster_id"}, {Name: "experiment_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"containers", "metrics_upload_at"}),
	}).Create(w)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}

func GetWorkloadsByClusterID(cluster_id uint) ([]Workload, error) {
	var workloads []Workload
	db := database.GetDB()
	err := db.Where("cluster_id = ?", cluster_id).Find(&workloads).Error
	return workloads, err
}

func WorkloadExistsByID(workload_id uint) bool {
	var workload Workload
	db := database.GetDB()
	err := db.First(&workload, workload_id).Error
	return err == nil
}
