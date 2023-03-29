package db

import (
	"fmt"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB = nil

func initDB() {
	cfg := config.GetConfig()
	log := logging.GetLogger()
	var (
		user     = cfg.DBUser
		password = cfg.DBPassword
		dbname   = cfg.DBName
		host     = cfg.DBHost
		port     = cfg.DBPort
		dbssl    = cfg.DBssl
	)
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=%s", user, password, dbname, host, port, dbssl)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	DB = db

	log.Info("DB initialization complete")
}

func GetDB() *gorm.DB {
	if DB == nil {
		initDB()
	}
	return DB
}
