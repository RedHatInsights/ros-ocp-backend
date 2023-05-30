package model

import (
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

type RHAccount struct {
	ID      uint   `gorm:"primaryKey;not null;autoIncrement"`
	Account string `gorm:"type:text;unique"`
	OrgId   string `gorm:"type:text;not null;unique"`
}

func (r *RHAccount) CreateRHAccount() error {
	db := database.GetDB()
	result := db.Where("org_id = ?", r.OrgId).FirstOrCreate(r)
	if result.Error != nil {
		dbError.Inc()
		return result.Error
	}
	if result.RowsAffected > 0 {
		rhAccountCreated.Inc()
	}
	return nil
}
