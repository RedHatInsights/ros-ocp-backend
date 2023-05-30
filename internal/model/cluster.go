package model

import (
	"time"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Cluster struct {
	ID                uint `gorm:"primaryKey;not null;autoIncrement"`
	TenantID          uint
	RHAccount         RHAccount `gorm:"foreignKey:TenantID"`
	SourceId          string    `gorm:"type:text;unique"`
	ClusterUUID       string    `gorm:"type:text;unique"`
	ClusterAlias      string    `gorm:"type:text;unique"`
	LastReportedAt    time.Time
	LastReportedAtStr string `gorm:"-"`
}

func (c *Cluster) AfterFind(tx *gorm.DB) error {
	c.LastReportedAtStr = c.LastReportedAt.Format(time.RFC3339)
	return nil
}

func (c *Cluster) CreateCluster() error {
	db := database.GetDB()
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "source_id"}, {Name: "cluster_uuid"}, {Name: "cluster_alias"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_reported_at"}),
	}).Create(c)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}
	return nil
}
