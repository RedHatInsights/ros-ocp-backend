package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
	"github.com/redhatinsights/ros-ocp-backend/internal/services"
)

var startCmd = &cobra.Command{Use: "start", Short: "Use to start ros-ocp-backend services"}

var processorCmd = &cobra.Command{
	Use:   "processor",
	Short: "starts ros-ocp processor",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp processor")
		cfg := config.GetConfig()
		processor.Setup_kruize_performance_profile()
		kafka.StartConsumer(cfg.UploadTopic, processor.ProcessReport)
	},
}

var recommenderCmd = &cobra.Command{
	Use:   "recommender",
	Short: "starts ros-ocp recommender service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp recommender service")
		cfg := config.GetConfig()
		processor.Setup_kruize_performance_profile()
		kafka.StartConsumer(cfg.ExperimentsTopic, services.ProcessEvent)
	},
}

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "starts ros-ocp api server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp api server")
		// Placeholder for starting api server code.
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.AddCommand(processorCmd)
	startCmd.AddCommand(recommenderCmd)
	startCmd.AddCommand(apiCmd)
}
