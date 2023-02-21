package model

type Cluster struct {
	ID        uint      `gorm:"primaryKey;not null;autoIncrement"`
	TenantID  string    `gorm:"type:text"`
	RHAccount RHAccount `gorm:"foreignKey:TenantID"`
	ClusterID string    `gorm:"type:text;unique"`
}
