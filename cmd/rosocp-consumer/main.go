package main

import (
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
)

func main() {
	config.InitConfig()
	logging.InitLogger()
	processor.Setup_kruize_performance_profile()
	kafka.StartConsumer()
}
