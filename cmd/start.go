package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
)

var startCmd = &cobra.Command{Use: "start", Short: "Use to start ros-ocp-backend services"}

var processorCmd = &cobra.Command{
	Use:   "processor",
	Short: "starts ros-ocp processor",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp processor")
		logging.InitLogger()
		processor.Setup_kruize_performance_profile()
		kafka.StartConsumer()
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
	startCmd.AddCommand(apiCmd)
}
