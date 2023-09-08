package utils

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

func Setup_kruize_performance_profile() {
	// This func needs to be revisited once kruize implements this API
	// Refer - https://github.com/kruize/autotune/blob/mvp_demo/src/main/java/com/autotune/analyzer/Analyzer.java#L50
	list_performance_profile_url := cfg.KruizeUrl + "/listPerformanceProfiles"
	for i := 0; i < 5; i++ {
		log.Infof("Fetching performance profile list")
		response, err := http.Get(list_performance_profile_url)
		if err != nil {
			log.Errorf("An Error Occured %v \n", err)
		} else {
			defer response.Body.Close()
			create_performance_profile_url := cfg.KruizeUrl + "/createPerformanceProfile"
			postBody, err := os.ReadFile("./resource_optimization_openshift.json")
			if err != nil {
				log.Errorf("File reading error: %v \n", err)
				os.Exit(1)
			}
			res, e := http.Post(create_performance_profile_url, "application/json", bytes.NewBuffer(postBody))
			if e != nil {
				log.Errorf("unable to create performance profile in kruize: %v \n", e)
			}
			defer res.Body.Close()
			if res.StatusCode == 201 {
				log.Infof("Performance profile created successfully")
				return
			}
			if res.StatusCode == 409 {
				log.Infof("Performance Profile already exist")
				return
			}
			bodyBytes, _ := io.ReadAll(res.Body)
			data := map[string]interface{}{}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				log.Errorf("can not unmarshal response data: %v", err)
				os.Exit(1)
			}
		}
		log.Infof("sleeping for 10 Seconds")
		time.Sleep(10 * time.Second)
	}

}

func ReadCSVFromUrl(url string) ([][]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	data, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return data, nil
}

type uniqueTypes interface {
	int | float64 | string
}

func unique[T uniqueTypes](x []T) []T {
	keys := make(map[T]bool)
	list := []T{}
	for _, entry := range x {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func Convert2DarrayToMap(arr [][]string) []map[string]interface{} {
	data := []map[string]interface{}{}
	for i := 1; i < len(arr); i++ {
		m := make(map[string]interface{})
		for j := 0; j < len(arr[0]); j++ {
			if metric, err := strconv.ParseFloat(arr[i][j], 64); err == nil {
				m[arr[0][j]] = metric
			} else {
				m[arr[0][j]] = arr[i][j]
			}
		}
		data = append(data, m)
	}
	return data
}

func ConvertDateToISO8601(date string) string {
	const date_format = "2006-01-02 15:04:05 -0700 MST"
	t, _ := time.Parse(date_format, date)
	return t.Format("2006-01-02T15:04:05.000Z")
}

func ConvertStringToTime(data string) (time.Time, error) {
	dateTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", data)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to convert string to time: %s", err)
	}
	return dateTime, nil

}

func ConvertISO8601StringToTime(data string) (time.Time, error) {
	dateTime, err := time.Parse("2006-01-02T15:04:05.000Z", data)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to convert string to time: %s", err)
	}
	return dateTime, nil
}

func MaxIntervalEndTime(slice []string) (time.Time, error) {
	var converted_date_slice []time.Time
	for _, v := range slice {
		formated_date, err := ConvertStringToTime(v)
		if err != nil {
			return time.Time{}, fmt.Errorf("unable to convert string to time in a slice: %s", err)
		}
		converted_date_slice = append(converted_date_slice, formated_date)

	}
	var max time.Time
	max = converted_date_slice[0]
	for _, ele := range converted_date_slice {
		if max.Before(ele) {
			max = ele
		}
	}
	return max, nil
}

func findInStringSlice(str string, s []string) int {
	for i, e := range s {
		if e == str {
			return i
		}
	}
	return -1
}

func GenerateExperimentName(org_id, source_id, cluster_id, namespace, k8s_object_type, k8s_object_name string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", org_id, source_id, cluster_id, namespace, k8s_object_type, k8s_object_name)

}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func Start_prometheus_server() {
	if cfg.PrometheusPort != "" {
		log.Info("Starting prometheus http server")
		http.Handle("/metrics", promhttp.Handler())
		_ = http.ListenAndServe(fmt.Sprintf(":%s", cfg.PrometheusPort), nil)
	}
}

func SliceK8sObjectToChunks(k8s_objects []map[string]interface{}) [][]map[string]interface{} {
	var chunks [][]map[string]interface{}
	chunkSize := cfg.KruizeMaxBulkChunkSize
	for i := 0; i < len(k8s_objects); i += chunkSize {
		end := i + chunkSize

		if end > len(k8s_objects) {
			end = len(k8s_objects)
		}

		chunks = append(chunks, k8s_objects[i:end])
	}

	return chunks
}
