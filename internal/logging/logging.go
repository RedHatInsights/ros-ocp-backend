package logging

import (
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"             //nolint:staticcheck
	"github.com/aws/aws-sdk-go/aws/credentials" //nolint:staticcheck
	lc "github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
	"github.com/sirupsen/logrus"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
)

var logger *logrus.Logger = nil
var log *logrus.Entry = nil

func initLogger() {
	logger = logrus.New()
	cfg := config.GetConfig()
	var logLevel logrus.Level

	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = logrus.DebugLevel
	case "ERROR":
		logLevel = logrus.ErrorLevel
	default:
		logLevel = logrus.InfoLevel
	}

	if cfg.LogFormater == "text" {
		logger.Formatter = &logrus.TextFormatter{}
	} else {
		logger.Formatter = &logrus.JSONFormatter{}
	}

	logger.Level = logLevel
	logger.Out = os.Stdout
	logger.ReportCaller = true

	if cfg.CwAccessKey != "" {
		cred := credentials.NewStaticCredentials(cfg.CwAccessKey, cfg.CwSecretKey, "")
		awsconf := aws.NewConfig().WithRegion(cfg.CwRegion).WithCredentials(cred)
		hook, err := lc.NewBatchingHook(cfg.CwLogGroup, cfg.CwLogStream, awsconf, 10*time.Second)
		if err != nil {
			logger.Info(err)
		}
		logger.Hooks.Add(hook)
	}
	log = logger.WithField("service", cfg.ServiceName)
}

func GetLogger() *logrus.Entry {
	if log == nil {
		initLogger()
		log.Info("Logging initialized")
		return log
	}
	return log
}

func Set_request_details(data types.KafkaMsg) *logrus.Entry {
	log = log.WithFields(logrus.Fields{
		"request_id":    data.Request_id,
		"account":       data.Metadata.Account,
		"org_id":        data.Metadata.Org_id,
		"source_id":     data.Metadata.Source_id,
		"cluster_uuid":  data.Metadata.Cluster_uuid,
		"cluster_alias": data.Metadata.Cluster_alias,
	})
	return log
}

func Set_request_details_recommendations(data types.RecommendationKafkaMsg) *logrus.Entry {
	log = log.WithFields(logrus.Fields{
		"request_id":         data.Request_id,
		"org_id":             data.Metadata.Org_id,
		"workload_id":        data.Metadata.Workload_id,
		"max_endtime_report": data.Metadata.Max_endtime_report,
		"experiment_name":    data.Metadata.Experiment_name,
	})
	return log
}
