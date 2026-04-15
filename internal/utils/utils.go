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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

// HTTPClient is the shared HTTP client for lightweight outbound requests
// (health checks, RBAC, experiment creation). The timeout is driven by
// GLOBAL_HTTP_CLIENT_TIMEOUT_SECS (default 30s) to prevent indefinite
// hangs when downstream services are slow or unresponsive. See FLPATH-3407.
//
// Heavy Kruize calls (/updateResults, /updateRecommendations) and large
// downloads (ReadCSVFromUrl) intentionally use the default http client
// until we have Prometheus latency data to set informed timeouts.
// TODO(FLPATH-3407): add per-endpoint Prometheus histogram to measure
// Kruize API latency, then set per-call timeouts:
//
//	kruizeAPIDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
//	    Name:    "rosocp_kruize_api_duration_seconds",
//	    Help:    "Latency of outbound Kruize API calls in seconds",
//	    Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120, 300},
//	}, []string{"path"})
var HTTPClient = newHTTPClient(cfg.GlobalHTTPClientTimeoutSecs)

const minHTTPTimeoutSecs = 1

func newHTTPClient(timeoutSecs int) *http.Client {
	if timeoutSecs < minHTTPTimeoutSecs {
		log.Warnf("GLOBAL_HTTP_CLIENT_TIMEOUT_SECS=%d is below minimum; using %ds", timeoutSecs, minHTTPTimeoutSecs)
		timeoutSecs = minHTTPTimeoutSecs
	}
	return &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}
}

func SetupKruizePerformanceProfile() {
	// This func needs to be revisited once kruize implements this API
	// Refer - https://github.com/kruize/autotune/blob/mvp_demo/src/main/java/com/autotune/analyzer/Analyzer.java#L50
	listPerformanceProfileUrl := cfg.KruizeUrl + "/listPerformanceProfiles"
	// Use the target version from config
	targetVersion := cfg.KruizePerformanceProfileVersion

	for i := 0; i < 5; i++ {
		log.Infof("fetching performance profile list")
		response, err := HTTPClient.Get(listPerformanceProfileUrl)
		if err != nil {
			log.Errorf("an error occurred %v \n", err)
		} else {
			body, err := io.ReadAll(response.Body)
			if respBodyErr := response.Body.Close(); respBodyErr != nil {
				log.Errorf("error closing response body: %v", respBodyErr)
			}
			if err != nil {
				log.Errorf("error reading listPerformanceProfiles response: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if len(body) > 0 {
				var profiles []map[string]interface{}
				if err := json.Unmarshal(body, &profiles); err != nil {
					log.Errorf("error unmarshalling listPerformanceProfiles response: %v", err)
				} else if len(profiles) > 0 {
					var fetchedVersion string
					for _, profile := range profiles {
						log.Debugf("current performance profile version : %v", profile["profile_version"])
						fetchedVersion = fmt.Sprintf("%v", profile["profile_version"])
					}

					// Convert versions to float64 for comparison
					fetchedVersionFloat, fetchedErr := strconv.ParseFloat(fetchedVersion, 64)
					targetVersionFloat, targetErr := strconv.ParseFloat(targetVersion, 64)

					if fetchedErr != nil || targetErr != nil {
						log.Errorf("failed to parse version numbers for comparison (fetched: %v, target: %v)", fetchedVersion, targetVersion)
						return
					}

					// Check if already up to date
					if fetchedVersionFloat == targetVersionFloat {
						log.Infof("performance profile already up to date (version: %v)", fetchedVersion)
						return
					}

					// Version mismatch -> Update the profile if update flag is enabled
					// and the fetched version is less than the target version (prevent downgrades)
					if cfg.UpdateKruizePerfProfile && fetchedVersionFloat < targetVersionFloat {
						log.Infof("Updating performance profile to supported version: %v", targetVersion)
						postBody, err := os.ReadFile("./resource_optimization_openshift.json")
						if err != nil {
							log.Errorf("file reading error: %v \n", err)
						}

						// create the PUT request
						updatePerformanceProfileUrl := cfg.KruizeUrl + "/updatePerformanceProfile"
						req, err := http.NewRequest(http.MethodPut, updatePerformanceProfileUrl, bytes.NewReader(postBody))
						if err != nil {
							log.Errorf("failed to create PUT request: %v", err)
							return
						}
						req.Header.Set("Content-Type", "application/json")

						// call the updatePerformanceProfile API using PUT request
						log.Debugf("sending PUT request to: %s (len=%d bytes)", updatePerformanceProfileUrl, req.ContentLength)
						res, err := HTTPClient.Do(req)
						if err != nil {
							log.Errorf("PUT request failed: %v", err)
							return
						}
						defer func() {
							if respBodyErr := res.Body.Close(); respBodyErr != nil {
								log.Errorf("error closing response body: %v", respBodyErr)
							}
						}()

						bodyBytes, _ := io.ReadAll(res.Body)
						log.Debugf("response status: %d", res.StatusCode)
						log.Debugf("response body: %s", string(bodyBytes))

						if res.StatusCode == 201 {
							log.Infof("performance profile updated successfully from %v to %v", fetchedVersion, targetVersion)
							return
						}
						log.Errorf("failed to update performance profile (status=%d): %s", res.StatusCode, targetVersion)
					}
				}
			}

			// If profile list empty or not found -> create new profile
			createPerformanceProfileUrl := cfg.KruizeUrl + "/createPerformanceProfile"
			log.Infof("creating new performance profile...")
			postBody, err := os.ReadFile("./resource_optimization_openshift.json")
			if err != nil {
				log.Errorf("File reading error: %v \n", err)
				os.Exit(1)
			}
			res, e := HTTPClient.Post(createPerformanceProfileUrl, "application/json", bytes.NewBuffer(postBody))
			if e != nil {
				log.Errorf("unable to create performance profile in kruize: %v \n", e)
			}
			defer func() {
				_ = res.Body.Close()
			}()
			if res.StatusCode == 201 {
				log.Infof("performance profile created successfully")
				return
			}
			if res.StatusCode == 409 {
				log.Infof("performance profile already exist")
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
	// TODO(FLPATH-3407): use a bounded client once we have latency data for CSV downloads
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()
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

func GenerateNamespaceExperimentName(org_id, source_id, cluster_id, namespace string) string {
	return fmt.Sprintf("%s|%s|%s|namespace|%s", org_id, source_id, cluster_id, namespace)
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

func NeedRecommOnFirstOfMonth(dbDate time.Time, maxEndTime time.Time) bool {
	if isItFirstOfMonth(maxEndTime) && getDate(maxEndTime).After(getDate(dbDate)) {
		return true
	}
	return false
}

func getDate(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

func isItFirstOfMonth(d time.Time) bool {
	_, _, day := d.Date()
	return day == 1
}

func DetermineCSVType(fileName string) types.PayloadType {
	isNamespace := strings.Contains(fileName, "namespace")

	if isNamespace {
		return types.PayloadTypeNamespace
	}
	return types.PayloadTypeContainer
}
