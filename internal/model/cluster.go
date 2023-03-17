package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/gorm/clause"
)

type Cluster struct {
	ID             uint `gorm:"primaryKey;not null;autoIncrement"`
	TenantID       uint
	RHAccount      RHAccount `gorm:"foreignKey:TenantID"`
	ClusterID      string    `gorm:"type:text;unique"`
	LastReportedAt time.Time
}

func (c *Cluster) CreateCluster() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "cluster_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_reported_at"}),
	}).Create(c)

	if result.Error != nil {
		return result.Error
	}
	return nil
}
