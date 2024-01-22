package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/api"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/services"
	"github.com/redhatinsights/ros-ocp-backend/internal/services/housekeeper"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
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

var recommendationPollerCmd = &cobra.Command{
	Use:   "recommendation-poller",
	Short: "starts ros-ocp recommendation-poller",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting ros-ocp recommendation-poller")
		cfg := config.GetConfig()
		go utils.Start_prometheus_server()
		kafka.StartConsumer(cfg.RecommendationTopic, services.PollForRecommendations, false)
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
		sourcesFlag, _ := cmd.Flags().GetBool("sources")
		partitionFlag, _ := cmd.Flags().GetBool("partition")
		if sourcesFlag {
			housekeeper.StartSourcesListenerService()
		}
		if partitionFlag {
			housekeeper.DeletePartitions()
		}
	},
}

var sources bool
var partition bool

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.AddCommand(processorCmd)
	startCmd.AddCommand(recommendationPollerCmd)
	startCmd.AddCommand(apiCmd)
	startCmd.AddCommand(houseKeeperCmd)

	houseKeeperCmd.Flags().BoolVar(&sources, "sources", false, "starts sources listener service")
	houseKeeperCmd.Flags().BoolVar(&partition, "partition", false, "deletes older partition")
	houseKeeperCmd.MarkFlagsOneRequired("sources", "partition")
	houseKeeperCmd.MarkFlagsMutuallyExclusive("sources", "partition")
}
