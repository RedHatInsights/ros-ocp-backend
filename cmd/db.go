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
			ExperimentName: "exfoo",
			Namespace:      "a_proj_rxu",
			WorkloadType:   workload.Replicaset,
			WorkloadName:   "replicaset_proj_rxu",
			Containers:     []string{"node", "postgres", "apache"},
		}
		db.Where(&model.Workload{Namespace: "a_proj_rxu"}).FirstOrCreate(&workload1)

		workload2 := &model.Workload{
			Cluster:        *cluster1,
			ExperimentName: "exbar",
			Namespace:      "b_proj_rxu",
			WorkloadType:   workload.Statefulset,
			WorkloadName:   "stateful_proj_rxu",
			Containers:     []string{"redis", "nginx"},
		}
		db.Where(&model.Workload{WorkloadType: workload.Statefulset}).FirstOrCreate(&workload2)

		workload3 := &model.Workload{
			Cluster:        *cluster2,
			ExperimentName: "exapp",
			Namespace:      "c_proj_rxu",
			WorkloadType:   workload.Deployment,
			WorkloadName:   "deployment_proj_rxu",
			Containers:     []string{"node", "postgres", "apache"},
		}
		db.Where(&model.Workload{WorkloadType: workload.Deployment}).FirstOrCreate(&workload3)

		recommendationSetData1 := map[string]interface{}{
			"duration_based": map[string]interface{}{
				"long_term": map[string]interface{}{
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
						"requests": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
					},
					"notifications": []map[string]interface{}{
						{
							"type":    "info",
							"message": "There is not enough data available to generate a recommendation.",
						},
					},
					"monitoring_end_time":   "0001-01-01T00:00:00Z",
					"monitoring_start_time": "0001-01-01T00:00:00Z",
				},
				"short_term": map[string]interface{}{
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 0.06,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 513900544,
								"format": "bytes",
							},
						},
						"requests": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 0.05,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 493311537.55,
								"format": "bytes",
							},
						},
					},
					"duration_in_hours":     0.23333333333333334,
					"monitoring_end_time":   "2023-04-02T00:15:00Z",
					"monitoring_start_time": "2023-04-01T00:15:00Z",
				},
				"medium_term": map[string]interface{}{
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
						"requests": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
					},
					"notifications": []map[string]interface{}{
						{
							"type":    "info",
							"message": "There is not enough data available to generate a recommendation.",
						},
					},
					"monitoring_end_time":   "0001-01-01T00:00:00Z",
					"monitoring_start_time": "0001-01-01T00:00:00Z",
				},
			},
			"workload":      "servers",
			"workload_type": "deployment",
		}

		recommendationSetData2 := map[string]interface{}{
			"duration_based": map[string]interface{}{
				"long_term": map[string]interface{}{
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
						"requests": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
					},
					"notifications": []map[string]interface{}{
						{
							"type":    "info",
							"message": "There is not enough data available to generate a recommendation.",
						},
					},
					"monitoring_end_time":   "0001-01-01T00:00:00Z",
					"monitoring_start_time": "0001-01-01T00:00:00Z",
				},
				"short_term": map[string]interface{}{
					"variation": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 0.06,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 513900544,
								"format": "bytes",
							},
						},
						"requests": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 0.578933223234234234,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 493311537868,
								"format": "bytes",
							},
						},
					},
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 5.3678942352345234523424,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 513900544,
								"format": "bytes",
							},
						},
						"requests": map[string]interface{}{
							"cpu": map[string]interface{}{
								"amount": 4.70,
								"format": "cores",
							},
							"memory": map[string]interface{}{
								"amount": 493311537.55,
								"format": "bytes",
							},
						},
					},
					"duration_in_hours":     0.23333333333333334,
					"monitoring_end_time":   "2023-04-02T00:15:00Z",
					"monitoring_start_time": "2023-04-01T00:15:00Z",
				},
				"medium_term": map[string]interface{}{
					"config": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
						"requests": map[string]interface{}{
							"cpu":    map[string]interface{}{},
							"memory": map[string]interface{}{},
						},
					},
					"notifications": []map[string]interface{}{
						{
							"type":    "info",
							"message": "There is not enough data available to generate a recommendation.",
						},
					},
					"monitoring_end_time":   "0001-01-01T00:00:00Z",
					"monitoring_start_time": "0001-01-01T00:00:00Z",
				},
			},
			"workload":      "servers",
			"workload_type": "replicaset",
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
		db.Where(&model.RecommendationSet{Recommendations: jsonrecommendationSetData1}).FirstOrCreate(&recommendationSet1)

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
			Workload:            *workload1,
			ContainerName:       "hadoop",
			MonitoringStartTime: time.Now().Add(-time.Hour * 3),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 2),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData2),
			UpdatedAt:           time.Now(),
		}
		db.Where(&model.RecommendationSet{ContainerName: "hadoop"}).FirstOrCreate(&recommendationSet3)

		recommendationSet4 := &model.RecommendationSet{
			Workload:            *workload2,
			ContainerName:       "nginx",
			MonitoringStartTime: time.Now().Add(-time.Hour * 3),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 2),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData2),
			UpdatedAt:           time.Now(),
		}
		db.Where(&model.RecommendationSet{ContainerName: "nginx"}).FirstOrCreate(&recommendationSet4)

		recommendationSet5 := &model.RecommendationSet{
			Workload:            *workload3,
			ContainerName:       "redis",
			MonitoringStartTime: time.Now().Add(-time.Hour * 3),
			MonitoringEndTime:   time.Now().Add(-time.Hour * 2),
			Recommendations:     datatypes.JSON(jsonrecommendationSetData2),
			UpdatedAt:           time.Now(),
		}
		db.Where(&model.RecommendationSet{ContainerName: "redis"}).FirstOrCreate(&recommendationSet5)

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
