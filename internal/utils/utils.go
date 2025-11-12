package utils

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	// Read the incoming JSON file once to get the version
	profileData, err := os.ReadFile("./resource_optimization_openshift.json")
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}

	var profile map[string]interface{}
	if err := json.Unmarshal(profileData, &profile); err != nil {
		log.Fatalf("Error unmarshalling new profile JSON: %v", err)
	}
	newVersion := profile["profile_version"]

	for i := 0; i < 5; i++ {
		log.Infof("Fetching performance profile list")
		response, err := http.Get(list_performance_profile_url)
		if err != nil {
			log.Errorf("An Error Occured %v \n", err)
		} else {
			body, err := io.ReadAll(response.Body)
			if respBodyErr := response.Body.Close(); respBodyErr != nil {
				log.Errorf("Error closing response body: %v", respBodyErr)
			}
			if err != nil {
				log.Errorf("Error reading listPerformanceProfiles response: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if len(body) > 0 {
				var profiles []map[string]interface{}
				if err := json.Unmarshal(body, &profiles); err != nil {
					log.Errorf("Error unmarshalling listPerformanceProfiles response: %v", err)
				} else if len(profiles) > 0 {
					for _, profile := range profiles {
						log.Infof("Current Performance Profile version : %v", profile["profile_version"])
						versionStr := profile["profile_version"]
						if versionStr == newVersion {
							log.Infof("Performance profile already up to date (version: %v)", versionStr)
							return
						}
					}

					// Version mismatch -> Update the profile if update flag is enabled
					if cfg.UpdateKruizePerfProfile {
						log.Infof("Updating performance profile to supported version: %v", newVersion)
						// ✅ Ensure Kruize PUT endpoint is ready before attempting update
						if !waitForKruizePutReady(cfg.KruizeUrl, 5, 30*time.Second) {
							log.Error("❌ Kruize PUT endpoint did not become ready — skipping update attempt.")
							return
						}
						postBody, err := os.ReadFile("./resource_optimization_openshift.json")
						if err != nil {
							log.Fatalf("File reading error: %v (path=%s)", err, postBody)
						}

						// create the PUT request
						update_performance_profile_url := cfg.KruizeUrl + "/updatePerformanceProfile"
						req, err := http.NewRequest(http.MethodPut, update_performance_profile_url, bytes.NewReader(postBody))
						if err != nil {
							log.Errorf("Failed to create PUT request: %v", err)
							return
						}
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("Accept", "*/*")

						// call the updatePerformanceProfile API using PUT request
						log.Infof("Sending PUT request to: %s (len=%d bytes)", update_performance_profile_url, req.ContentLength)
						res, err := http.DefaultClient.Do(req)
						if err != nil {
							log.Errorf("PUT request failed: %v", err)
							return
						}
						defer func() {
							if respBodyErr := res.Body.Close(); respBodyErr != nil {
								log.Errorf("Error closing response body: %v", respBodyErr)
							}
						}()

						bodyBytes, _ := io.ReadAll(res.Body)
						log.Infof("Response status: %d", res.StatusCode)
						log.Infof("Response body: %s", string(bodyBytes))

						if res.StatusCode == 201 {
							log.Infof("Performance profile updated successfully.")
							return
						}
						log.Errorf("Failed to update performance profile (status=%d): %s", res.StatusCode, string(bodyBytes))
					}
				}
			}

			// If profile list empty or not found -> create new profile
			create_performance_profile_url := cfg.KruizeUrl + "/createPerformanceProfile"
			log.Infof("Creating new performance profile...")
			postBody, err := os.ReadFile("./resource_optimization_openshift.json")
			if err != nil {
				log.Errorf("File reading error: %v \n", err)
				os.Exit(1)
			}
			res, e := http.Post(create_performance_profile_url, "application/json", bytes.NewBuffer(postBody))
			if e != nil {
				log.Errorf("unable to create performance profile in kruize: %v \n", e)
			}
			defer func() {
				_ = res.Body.Close()
			}()
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

// waitForKruizePutReady ensures the /updatePerformanceProfile endpoint is ready to accept PUTs.
// It polls the endpoint using an OPTIONS request and checks if the Allow header includes PUT.
func waitForKruizePutReady(kruizeURL string, retries int, delay time.Duration) bool {
	target := kruizeURL + "/updatePerformanceProfile"
	log.Infof("Checking Kruize PUT endpoint readiness at: %s", target)

	for i := 1; i <= retries; i++ {
		req, _ := http.NewRequest(http.MethodOptions, target, nil)
		client := &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives:  true,
				DisableCompression: true,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 0,
				}).DialContext,
			},
			Timeout: 10 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Warnf("Attempt %d/%d: OPTIONS check failed: %v", i, retries, err)
		} else {
			err := resp.Body.Close()
			if err != nil {
				return false
			}
			allow := resp.Header.Get("Allow")
			if strings.Contains(allow, "PUT") {
				log.Info("✅ Kruize PUT endpoint is ready!")
				return true
			}
		}
		log.Infof("Waiting %v before retry...", delay)
		time.Sleep(delay)
	}
	log.Error("❌ Kruize PUT endpoint not ready after multiple attempts.")
	return false
}

func ReadCSVFromUrl(url string) ([][]string, error) {
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
