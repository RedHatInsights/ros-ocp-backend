package model

import (
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

type RHAccount struct {
	ID      uint   `gorm:"primaryKey;not null;autoIncrement"`
	Account string `gorm:"type:text;unique"`
	OrgId   string `gorm:"type:text;unique"`
}

func (r *RHAccount) CreateRHAccount() error {
	db := database.GetDB()
	result := db.Where("account = ? OR org_id = ?", r.Account, r.OrgId).FirstOrCreate(r)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
