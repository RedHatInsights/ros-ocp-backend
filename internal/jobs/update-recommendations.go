package jobs

/*
This job looks for missing recommendations, requests and saves them
Originally intended for integration with Kruize v20
*/

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

var log = logging.GetLogger()
var cfg *config.Config = config.GetConfig()
var db = database.GetDB()

var (
	invalidRecommendationUpdateRecommendationJob = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rosocp_invalid_recommendation_update_recommendation_job",
		Help: "Invalid recommendations sum from Kruize gathered while executing update-recommendations job",
	})
)

func checkURLStatus() bool {
	kruizeUrl := cfg.KruizeUrl + "/updateRecommendations"
	_, err := url.ParseRequestURI(kruizeUrl)
	return err == nil
}

func UpdateRecommendations() {

	maxRetries := 10
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		if checkURLStatus() {
			RenewRecommendations()
			break
		} else {
			fmt.Printf("/updateRecommendations is down: (Retry %d of %d)\n", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

}

func RenewRecommendations() {

	var workloads []model.Workload
	query := fmt.Sprintf("SELECT id, experiment_name, metrics_uploaded_at FROM %s", "workloads")
	if err := db.Raw(query).Scan(&workloads).Error; err != nil {
		log.Fatal(err)
	}

	// Process the results
	for _, workload := range workloads {

		recommendationSets, err := model.GetFirstRecommendationSetsByWorkloadID(workload.ID)

		if err != nil {
			panic(err.Error())
		}

		if reflect.ValueOf(recommendationSets).IsZero() {

			experiment_name := workload.ExperimentName
			maxEndTime := workload.MetricsUploadAt

			recommendation, err := kruize.Update_recommendations(experiment_name, maxEndTime)
			if err != nil {
				end_interval := utils.ConvertDateToISO8601(maxEndTime.String())
				if err.Error() == fmt.Sprintf("Recommendation for timestamp - \" %s \" does not exist", end_interval) {
					log.Infof("Recommendation does not exist for timestamp - \" %s \"", end_interval)
					continue
				}
				log.Errorf("Unable to list recommendation for: %v", err)
				continue
			}

			if kruize.Is_valid_recommendation(recommendation) {
				containers := recommendation[0].Kubernetes_objects[0].Containers
				for _, container := range containers {
					for _, v := range container.Recommendations.Data {
						marshalData, err := json.Marshal(v)
						if err != nil {
							log.Errorf("Unable to list recommendation for: %v", err)
						}

						// Create RecommendationSet entry into the table.
						recommendationSet := model.RecommendationSet{
							WorkloadID:          workload.ID,
							ContainerName:       container.Container_name,
							MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
							MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
							Recommendations:     marshalData,
						}
						if err := recommendationSet.CreateRecommendationSet(); err != nil {
							log.Errorf("Unable to save a record into recommendation set: %v. Error: %v", recommendationSet, err)
							return
						} else {
							log.Infof("Recommendation saved for experiment - %s and end_interval - %s", experiment_name, recommendationSet.MonitoringEndTime)
						}

						// Create entry into HistoricalRecommendationSet table.
						historicalRecommendationSet := model.HistoricalRecommendationSet{
							OrgId:               workload.OrgId,
							WorkloadID:          workload.ID,
							ContainerName:       container.Container_name,
							MonitoringStartTime: v.Duration_based.Short_term.Monitoring_start_time,
							MonitoringEndTime:   v.Duration_based.Short_term.Monitoring_end_time,
							Recommendations:     marshalData,
						}
						if err := historicalRecommendationSet.CreateHistoricalRecommendationSet(); err != nil {
							log.Errorf("unable to get or add record to historical recommendation set table: %v. Error: %v", recommendationSet, err)
							return
						}
					}
				}
			} else {
				invalidRecommendationUpdateRecommendationJob.Inc()
			}
		}
	}

}
