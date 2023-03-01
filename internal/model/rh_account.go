package model

type RHAccount struct {
	ID      uint   `gorm:"primaryKey;not null;autoIncrement"`
	Account string `gorm:"type:text;unique"`
	OrgId   string `gorm:"type:text;unique"`
}
