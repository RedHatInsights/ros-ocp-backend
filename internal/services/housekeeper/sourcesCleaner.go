package housekeeper

import (
	"encoding/json"
	"os"
	"strconv"

	k "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/kruize"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils/sources"
)

// OnPremCostAppID is the default application ID used in on-prem deployments
const OnPremCostAppID = 0

var cost_app_id int

func StartSourcesListenerService() {
	log := logging.GetLogger()
	cfg := config.GetConfig()
	var err error

	if cfg.ONPREM {
		cost_app_id = OnPremCostAppID
		log.Infof("ONPREM flag is set, cost_app_id set to %d", OnPremCostAppID)
	} else {
		cost_app_id, err = sources.GetCostApplicationID()
		if err != nil {
			log.Error("Unable to get cost application id", err)
			os.Exit(1)
		}
	}

	kafka.StartConsumer(cfg.SourcesEventTopic, sourcesListener)
}

func sourcesListener(msg *k.Message, _ *k.Consumer) {
	db := database.GetDB()
	log := logging.GetLogger()
	headers := msg.Headers
	for _, v := range headers {
		if v.Key == "event_type" && string(v.Value) == "Application.destroy" {
			var data types.SourcesEvent
			if !json.Valid([]byte(msg.Value)) {
				log.Errorf("Received message on kafka topic is not vaild JSON: %s", msg.Value)
				return
			}
			if err := json.Unmarshal(msg.Value, &data); err != nil {
				log.Errorf("Unable to decode kafka message: %s", msg.Value)
				return
			}
			if data.Application_type_id == cost_app_id {
				var cluster model.Cluster
				db.Where("source_id = ?", strconv.Itoa(data.Source_id)).First(&cluster)
				workloads, err := model.GetWorkloadsByClusterID(cluster.ID)
				if err != nil {
					log.Errorf("unable to get workloads for cluster: %v. Error: %v", cluster, err)
					return
				}

				for _, workload := range workloads {
					kruize.Delete_experiment_from_kruize(workload.ExperimentName)
				}

				if err := cluster.DeleteCluster(); err != nil {
					log.Errorf("unable to delete record from clusters table: %v. Error: %v", cluster, err)
				} else {
					log.Infof("Successfully deleted the cluster with Source_id: %v.", cluster.SourceId)
				}
			}

		}
	}
}
