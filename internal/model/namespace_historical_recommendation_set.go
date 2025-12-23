package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HistoricalNamespaceRecommendationSet struct {
	ID                   uint   `gorm:"primaryKey;not null"`
	OrgID                string `gorm:"type:text;not null"`
	WorkloadID           uint   `gorm:"not null"`
	NamespaceName        string `gorm:"type:text;not null"`
	CPURequestCurrent    float64
	CPUVariation         float64
	MemoryRequestCurrent float64
	MemoryVariation      float64
	MonitoringStartTime  time.Time `gorm:"type:timestamp with time zone;not null"`
	MonitoringEndTime    time.Time `gorm:"type:timestamp with time zone;not null"`
	Recommendations      datatypes.JSON
	CreatedAt            time.Time `gorm:"type:timestamp with time zone;not null"`
	UpdatedAt            time.Time `gorm:"type:timestamp with time zone;not null"`
}

func (h *HistoricalNamespaceRecommendationSet) CreateHistoricalRecommendationSet(tx *gorm.DB) error {
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "workload_id"}, {Name: "monitoring_end_time"}},
		DoNothing: true,
	}).Create(h)

	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}

	return nil
}
