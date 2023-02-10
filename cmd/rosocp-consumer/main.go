package main

import (
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

func main() {
	config.InitConfig()
	logging.InitLogger()
	kafka.StartConsumer()
}
