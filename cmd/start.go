package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/api"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/services"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/jobs"

)

var startCmd = &cobra.Command{Use: "start", Short: "Use to start ros-ocp-backend services"}

var processorCmd = &cobra.Command{
	Use:   "processor",
	Short: "starts ros-ocp processor",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp processor")
		cfg := config.GetConfig()
		go utils.Start_prometheus_server()
		utils.Setup_kruize_performance_profile()
		kafka.StartConsumer(cfg.UploadTopic, services.ProcessReport)
	},
}

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "starts ros-ocp api server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting ros-ocp API server")
		api.StartAPIServer()
	},
}

var houseKeeperCmd = &cobra.Command{
	Use:   "housekeeper",
	Short: "starts ros-ocp housekeeper service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp housekeeper service")
		services.StartHouseKeeperService()
	},
}

// One time job
var updateRecommendationsCmd = &cobra.Command{
	Use:   "update-recommendations",
	Short: "updates missing recommendations",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("updating missing recommendations using Kruize v20")
		jobs.UpdateRecommendations()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.AddCommand(processorCmd)
	startCmd.AddCommand(apiCmd)
	startCmd.AddCommand(houseKeeperCmd)
	startCmd.AddCommand(updateRecommendationsCmd)
}
