package kruizePayload

import (
	"fmt"
	"strconv"
	"time"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

type kubernetesObject struct {
	K8stype    string      `json:"type,omitempty"`
	Name       string      `json:"name,omitempty"`
	Namespace  string      `json:"namespace,omitempty"`
	Containers []container `json:"containers,omitempty"`
}

type container struct {
	Container_image_name string         `json:"container_image_name,omitempty"`
	Container_name       string         `json:"container_name,omitempty"`
	Metrics              []metric       `json:"metrics,omitempty"`
	Recommendations      Recommendation `json:"recommendations,omitempty"`
}

type metric struct {
	Name    string `json:"name,omitempty"`
	Results result `json:"results,omitempty"`
}

type result struct {
	Aggregation_info aggregation_info `json:"aggregation_info,omitempty"`
}

type aggregation_info struct {
	Min    string `json:"min,omitempty"`
	Max    string `json:"max,omitempty"`
	Sum    string `json:"sum,omitempty"`
	Avg    string `json:"avg,omitempty"`
	Format string `json:"format,omitempty"`
}

type Recommendation struct {
	Version       string                        `json:"version,omitempty"`
	Data          map[string]RecommendationData `json:"data,omitempty"`
	Notifications map[string]Notification       `json:"notifications,omitempty"`
}

type Notification struct {
	NotifyType string `json:"type,omitempty"`
	Message    string `json:"message,omitempty"`
	Code       int    `json:"code,omitempty"`
}

type RecommendationEngineObject struct {
	PodsCount       int                     `json:"pods_count,omitempty"`
	ConfidenceLevel float64                 `json:"confidence_level,omitempty"`
	Config          ConfigObject            `json:"config,omitempty"`
	Variation       ConfigObject            `json:"variation,omitempty"`
	Notifications   map[string]Notification `json:"notifications,omitempty"`
}

type RecommendationData struct {
	Notifications       map[string]Notification `json:"notifications,omitempty"`
	MonitoringEndTime   time.Time               `json:"monitoring_end_time,omitempty"`
	Current             ConfigObject            `json:"current,omitempty"`
	RecommendationTerms Term                    `json:"recommendation_terms,omitempty"`
}

type RecommendationTerm struct {
	DurationInHours       float64                 `json:"duration_in_hours,omitempty"`
	Notifications         map[string]Notification `json:"notifications,omitempty"`
	MonitoringStartTime   time.Time               `json:"monitoring_start_time,omitempty"`
	RecommendationEngines *struct {
		Cost        RecommendationEngineObject `json:"cost,omitempty"`
		Performance RecommendationEngineObject `json:"performance,omitempty"`
	} `json:"recommendation_engines,omitempty"`
	Plots *Plot `json:"plots,omitempty"`
}

type Plot struct {
	DataPoints int                  `json:"datapoints,omitempty"`
	PlotsData  map[string]PlotsData `json:"plots_data,omitempty"`
}

type PlotsData struct {
	CpuUsage    *BoxPlotDetails `json:"cpuUsage,omitempty"`
	MemoryUsage *BoxPlotDetails `json:"memoryUsage,omitempty"`
}

type BoxPlotDetails struct {
	Min    float64 `json:"min,omitempty"`
	Q1     float64 `json:"q1,omitempty"`
	Median float64 `json:"median,omitempty"`
	Q3     float64 `json:"q3,omitempty"`
	Max    float64 `json:"max,omitempty"`
	Format string  `json:"format,omitempty"`
}

type Term struct {
	Short_term  RecommendationTerm `json:"short_term"`
	Medium_term RecommendationTerm `json:"medium_term"`
	Long_term   RecommendationTerm `json:"long_term,omitempty"`
}

type ConfigObject struct {
	Limits   recommendedConfig `json:"limits,omitempty"`
	Requests recommendedConfig `json:"requests,omitempty"`
}

type recommendedConfig struct {
	Cpu    recommendedValues `json:"cpu,omitempty"`
	Memory recommendedValues `json:"memory,omitempty"`
}

type recommendedValues struct {
	Amount float64 `json:"amount,omitempty"`
	Format string  `json:"format,omitempty"`
}

func AssertAndConvertToString(data interface{}) string {
	if metric, ok := data.(float64); ok {
		return fmt.Sprintf("%.2f", metric)
	}
	if metric, ok := data.(int); ok {
		return strconv.Itoa(metric)
	}
	if metric, ok := data.(string); ok {
		return metric
	}
	return ""

}

func make_container_data(c map[string]interface{}) container {
	metrics := []metric{}

	// Initialising a map with name Metrics Map
	// It holds the keys to get the required data to create a metrics instance
	metricsMap := map[string]map[string]string{
		// CPU Request
		"cpuRequest": {
			"sum":    "cpu_request_container_sum_SUM",
			"avg":    "cpu_request_container_avg_MEAN",
			"format": "cores",
		},
		// CPU Limit
		"cpuLimit": {
			"sum":    "cpu_limit_container_sum_SUM",
			"avg":    "cpu_limit_container_avg_MEAN",
			"format": "cores",
		},
		// CPU Usage
		"cpuUsage": {
			"sum":    "cpu_usage_container_sum_SUM",
			"avg":    "cpu_usage_container_avg_MEAN",
			"min":    "cpu_usage_container_min_MIN",
			"max":    "cpu_usage_container_max_MAX",
			"format": "cores",
		},
		// CPU Throttle
		"cpuThrottle": {
			"sum":    "cpu_throttle_container_sum_SUM",
			"avg":    "cpu_throttle_container_avg_MEAN",
			"max":    "cpu_throttle_container_max_MAX",
			"format": "cores",
		},
		// Memory Request
		"memoryRequest": {
			"sum":    "memory_request_container_sum_SUM",
			"avg":    "memory_request_container_avg_MEAN",
			"format": "Mi",
		},
		// Memory Limit
		"memoryLimit": {
			"sum":    "memory_limit_container_sum_SUM",
			"avg":    "memory_limit_container_avg_MEAN",
			"format": "Mi",
		},
		// Memory Usage
		"memoryUsage": {
			"sum":    "memory_usage_container_sum_SUM",
			"avg":    "memory_usage_container_avg_MEAN",
			"min":    "memory_usage_container_min_MIN",
			"max":    "memory_usage_container_max_MAX",
			"format": "Mi",
		},
		// Memory RSS
		"memoryRSS": {
			"sum":    "memory_rss_usage_container_sum_SUM",
			"avg":    "memory_rss_usage_container_avg_MEAN",
			"min":    "memory_rss_usage_container_min_MIN",
			"max":    "memory_rss_usage_container_max_MAX",
			"format": "Mi",
		},
	}

	// Initialising variable to hold the result of SUM
	sum := ""
	// Initailising variable to hold the result of MEAN
	avg := ""
	// Initialising variable to hold the result of MIN value
	min := ""
	// Initailising variable to hold the result of MAX value
	max := ""
	// Initialising variable to hold the result of FORMAT
	format := ""

	// Iterate over the map to create metric instances
	for metricName, metricFields := range metricsMap {

		// Check if "sum" key exists in map
		if sum_field, ok := metricFields["sum"]; ok {
			if metricFields["format"] == "Mi" {
				convertMemoryField(sum_field, c)
			}
			// Assign the sum value returned
			sum = AssertAndConvertToString(c[sum_field])
		} else {
			// Set "sum" to empty string (to get it skipped in json)
			sum = ""
		}

		// Check if "avg" key exists in map
		if avg_field, ok := metricFields["avg"]; ok {
			if metricFields["format"] == "Mi" {
				convertMemoryField(avg_field, c)
			}
			// Assign the avg value returned
			avg = AssertAndConvertToString(c[avg_field])
		} else {
			// Set "avg" to empty string (to get it skipped in json)
			avg = ""
		}

		// Check if "min" key exists in map
		if min_field, ok := metricFields["min"]; ok {
			if metricFields["format"] == "Mi" {
				convertMemoryField(min_field, c)
			}
			// Assign the min value returned
			min = AssertAndConvertToString(c[min_field])
		} else {
			// Set "min" to empty string (to get it skipped in json)
			min = ""
		}

		// Check if "max" key exists in map
		if max_field, ok := metricFields["max"]; ok {
			if metricFields["format"] == "Mi" {
				convertMemoryField(max_field, c)
			}
			// Assign the max value returned
			max = AssertAndConvertToString(c[max_field])
		} else {
			// Set "max" to empty string (to get it skipped in json)
			max = ""
		}

		// Check if "format" key exists in map
		if format_field, ok := metricFields["format"]; ok {
			// Assign the format value returned
			format = format_field
		} else {
			// Set "format" to empty string (to get it skipped in json)
			format = ""
		}

		// Check if "sum" & "avg" are not empty to proceed for metric creation
		if sum != "" && avg != "" {
			metrics = append(metrics, metric{
				Name: metricName,
				Results: result{
					Aggregation_info: aggregation_info{
						Sum:    sum,
						Avg:    avg,
						Min:    min,
						Max:    max,
						Format: format,
					},
				},
			})
		}
	}

	container_data := container{
		Container_image_name: c["image_name"].(string),
		Container_name:       c["container_name"].(string),
		Metrics:              metrics,
	}

	return container_data
}

func convertMemoryField(field string, c map[string]interface{}) {
	log := logging.GetLogger()
	var memoryInMi float64
	memField, ok := c[field].(float64)
	if ok {
		memoryInMi = memField / 1024 / 1024
	} else {
		log.Error("Failed to convert field: ", field)
		return
	}
	c[field] = memoryInMi
}
