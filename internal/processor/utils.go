package processor

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func Setup_kruize_performance_profile() {
	// This func needs to be revisited once kruize implements this API
	// Refer - https://github.com/kruize/autotune/blob/mvp_demo/src/main/java/com/autotune/analyzer/Analyzer.java#L50
	list_performance_profile_url := cfg.KruizeUrl + "/listPerformanceProfiles"
	for i := 0; i < 5; i++ {
		log.Infof("Fetching performance profile list")
		_, err := http.Get(list_performance_profile_url)
		if err != nil {
			log.Errorf("An Error Occured %v \n", err)
		} else {
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
			if res.StatusCode == 201 {
				log.Infof("Performance profile created successfully")
				return
			}
			defer res.Body.Close()
			bodyBytes, _ := io.ReadAll(res.Body)
			data := map[string]interface{}{}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				log.Errorf("can not unmarshal response data: %v", err)
				os.Exit(1)
			}
			if data["message"] == "Performance Profile name : resource-optimization-openshift is duplicate" {
				log.Infof("Performance Profile already exist")
				return
			}
		}
		log.Infof("sleeping for 10 Seconds")
		time.Sleep(10 * time.Second)
	}

}

func readCSVFromUrl(url string) ([][]string, error) {
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

func convert2DarrayToMap(arr [][]string) []map[string]interface{} {
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

func convertDateToISO8601(date string) string {
	const date_format = "2006-01-02 15:04:05 -0700 MST"
	t, _ := time.Parse(date_format, date)
	return t.Format(time.RFC3339)
}

func findInStringSlice(str string, s []string) int {
	for i, e := range s {
		if e == str {
			return i
		}
	}
	return -1
}
