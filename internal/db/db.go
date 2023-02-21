package db

import (
	"fmt"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	cfg := config.GetConfig()
	log := logging.GetLogger()
	var (
		user     = cfg.DBUser
		password = cfg.DBPassword
		dbname   = cfg.DBName
		host     = cfg.DBHost
		port     = cfg.DBPort
	)
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable", user, password, dbname, host, port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	DB = db

	log.Info("DB initialization complete")
}
