package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	"gorm.io/datatypes"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
)

func getMigrateInstance() *migrate.Migrate {
	cfg := config.GetConfig()
	rdsCA := database.CreateCACertFile(cfg.DBCACert)
	m, err := migrate.New(
		"file://./migrations",
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&sslrootcert=%s", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBssl, rdsCA))
	if err != nil {
		fmt.Printf("Unable to get migration instance: %v\n", err)
		os.Exit(1)
	}
	return m
}

var migrateCmd = &cobra.Command{Use: "migrate", Short: "migrate database"}

var migrateUp = &cobra.Command{
	Use:   "up",
	Short: "Forward database migration",
	Long:  "Forward database migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Forward database migration")
		m := getMigrateInstance()
		err := m.Up()
		if err != nil {
			fmt.Println(err)
		}
	},
}

var migratedown = &cobra.Command{
	Use:   "down",
	Short: "Reverse database migration",
	Long:  "Reverse database migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Reverse database migration")
		all, _ := cmd.Flags().GetBool("all")
		m := getMigrateInstance()
		var err error
		if all {
			err = m.Down()
		} else {
			err = m.Steps(-1)
		}
		if err != nil {
			fmt.Println(err)
		}
	},
}

var revision = &cobra.Command{
	Use:   "revision",
	Short: "Get details of database migration",
	Long:  "It pulls the record from schema_migrations table",
	Run: func(cmd *cobra.Command, args []string) {
		m := getMigrateInstance()
		version, dirty, err := m.Version()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Current migration version is: %v \n", version)
		fmt.Printf("Is dirty: %v \n", dirty)
	},
}

var seedCmd = &cobra.Command{
	Use:   "apiseedtest",
	Short: "seed database for local api testing",
	Long:  "seed database for local api testing",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("seed database")
		db := database.GetDB()

		// Changes for seeding API data; local testing

		rhAccount1 := &model.RHAccount{
			Account: "2234",
			OrgId:   "3340851",
		}
		db.FirstOrCreate(&rhAccount1)

		rhAccount2 := &model.RHAccount{
			Account: "22",
			OrgId:   "foo_org2",
		}
		db.FirstOrCreate(&rhAccount2)

		cluster1 := &model.Cluster{
			RHAccount:      *rhAccount1,
			ClusterUUID:    "db7cb483-b890-45c2-a803-d99a17eee205",
			ClusterAlias:   "FooAlias",
			LastReportedAt: time.Now().Add(-time.Hour * 3),
		}
		db.FirstOrCreate(&cluster1)

		cluster2 := &model.Cluster{
			RHAccount:      *rhAccount1,
			ClusterUUID:    "57e83fd6-9e4c-4de2-bb2b-24f543a4a600",
			ClusterAlias:   "BarAlias",
			LastReportedAt: time.Now().Add(-time.Hour * 2),
		}
		db.Where(&model.Cluster{ClusterAlias: "BarAlias"}).FirstOrCreate(&cluster2)

		workload1 := &model.Workload{
			Cluster:        *cluster1,
			ExperimentName: "exfoo1",
			Namespace:      "a_proj_rxu",
			WorkloadType:   workload.Replicaset,
			WorkloadName:   "replicaset_proj_rxu",
			Containers:     []string{"node", "postgres", "apache"},
		}
		db.FirstOrCreate(&workload1)

		workload2 := &model.Workload{
			Cluster:        *cluster2,
			ExperimentName: "exfoo2",
			Namespace:      "b_proj_rxu",
			WorkloadType:   workload.Deployment,
			WorkloadName:   "deployment_proj_rxu",
			Containers:     []string{"node", "postgres", "apache"},
		}
		db.Where(&model.Workload{WorkloadType: workload.Deployment}).FirstOrCreate(&workload2)

		recommendationSetData1 := map[string]interface{}{
			"interval": 15,
			"cpu": map[string]interface{}{
				"current": map[string]interface{}{
					"request": 5,
					"limit":   2,
				},
				"recommended": map[string]interface{}{
					"request": 7,
					"limit":   3,
					"delta":   2,
				},
			},
			"memory": map[string]interface{}{
				"current": map[string]interface{}{
					"request": 5,
					"limit":   3,
				},
				"recommended": map[string]interface{}{
					"request": 5,
					"limit":   2,
					"delta":   1,
				},
			},
			"reported": "24/12/1992",
		}

		recommendationSetData2 := map[string]interface{}{
			"interval": 7,
			"cpu": map[string]interface{}{
				"current": map[string]interface{}{
					"request": 51,
					"limit":   2,
				},
				"recommended": map[string]interface{}{
					"request": 7,
					"limit":   3,
					"delta":   2,
				},
			},
			"memory": map[string]interface{}{
				"current": map[string]interface{}{
					"request": 5,
					"limit":   32,
				},
				"recommended": map[string]interface{}{
					"request": 5,
					"limit":   2,
					"delta":   2,
				},
			},
			"reported": "01/02/1996",
		}

		jsonrecommendationSetData1, err := json.Marshal(recommendationSetData1)
		if err != nil {
			fmt.Print("unable to seed recommendation-set-1 data")
		}

		jsonrecommendationSetData2, err := json.Marshal(recommendationSetData2)
		if err != nil {
			fmt.Print("unable to seed recommendation-set-2 data")
		}

		recommendationSet1 := &model.RecommendationSet{
			Workload:            *workload1,
			ContainerName:       "postgres",
			MonitoringStartTime: time.Now().Add(-time.Hour * 3),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 2),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData1),
			UpdatedAt:           time.Now(),
		}
		db.FirstOrCreate(&recommendationSet1)

		recommendationSet2 := &model.RecommendationSet{
			Workload:            *workload1,
			ContainerName:       "postgres",
			MonitoringStartTime: time.Now().Add(-time.Hour * 2),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 1),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData2),
			UpdatedAt:           time.Now(),
		}
		db.Where(&model.RecommendationSet{Recommendations: jsonrecommendationSetData2}).FirstOrCreate(&recommendationSet2)

		recommendationSet3 := &model.RecommendationSet{
			Workload:            *workload2,
			ContainerName:       "nodejs",
			MonitoringStartTime: time.Now().Add(-time.Hour * 3),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 2),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData2),
			UpdatedAt:           time.Now(),
		}
		db.Where(&model.RecommendationSet{ContainerName: "nodejs"}).FirstOrCreate(&recommendationSet3)
	},
}

var dbCmd = &cobra.Command{Use: "db", Short: "Use to migrate or seed database"}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(migrateCmd)
	dbCmd.AddCommand(seedCmd)
	dbCmd.AddCommand(revision)
	migrateCmd.AddCommand(migrateUp)
	migrateCmd.AddCommand(migratedown)
	migratedown.Flags().Bool("all", false, "Used to undo all migrations")
}
