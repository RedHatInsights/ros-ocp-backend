package logging

import (
	"os"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = nil

func initLogger() {
	log = logrus.New()
	cfg := config.GetConfig()
	var logLevel logrus.Level

	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = logrus.DebugLevel
	case "ERROR":
		logLevel = logrus.ErrorLevel
	default:
		logLevel = logrus.InfoLevel
	}

	log.Level = logLevel
	log.Out = os.Stdout
	log.ReportCaller = true
}

func GetLogger() *logrus.Logger {
	if log == nil {
		initLogger()
		log.Info("Logging initialized")
		return log
	}
	return log
}
