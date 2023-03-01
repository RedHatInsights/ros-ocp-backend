package cmd

import (
	"fmt"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting database migration")
		config.InitConfig()
		db.InitDB()
		err := db.DB.AutoMigrate(
			&model.RHAccount{},
			&model.Cluster{},
			&model.Workload{},
			&model.RecommendationSet{},
		)
		if err != nil {
			fmt.Println("DB Migration Failed..")
		}
	},
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "seed database",
	Long:  "seed database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("seed database")
		// Placeholder for db seed code.
	},
}

var dbCmd = &cobra.Command{Use: "db", Short: "Use to migrate or seed database"}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(migrateCmd)
	dbCmd.AddCommand(seedCmd)
}
