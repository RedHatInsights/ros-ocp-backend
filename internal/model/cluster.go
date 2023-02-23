package model

import "time"

type Cluster struct {
	ID             uint `gorm:"primaryKey;not null;autoIncrement"`
	TenantID       uint
	RHAccount      RHAccount `gorm:"foreignKey:TenantID"`
	ClusterID      string    `gorm:"type:text;unique"`
	LastReportedAt time.Time
}
