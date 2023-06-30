package kruizePayload

import (
	"fmt"
	"time"
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
	Recommendations      recommendation `json:"recommendations,omitempty"`
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

type recommendation struct {
	Data          map[string]recommendationType `json:"data,omitempty"`
	Notifications map[string]notification       `json:"notifications,omitempty"`
}

type notification struct {
	NotifyType string `json:"type,omitempty"`
	Message    string `json:"message,omitempty"`
	Code       int    `json:"code,omitempty"`
}

type recommendationType struct {
	Duration_based termbased `json:"duration_based,omitempty"`
}

type termbased struct {
	Short_term  recommendationObject `json:"short_term,omitempty"`
	Medium_term recommendationObject `json:"medium_term,omitempty"`
	Long_term   recommendationObject `json:"long_term,omitempty"`
}

type recommendationObject struct {
	Monitoring_start_time time.Time               `json:"monitoring_start_time,omitempty"`
	Monitoring_end_time   time.Time               `json:"monitoring_end_time,omitempty"`
	Duration_in_hours     float64                 `json:"duration_in_hours,omitempty"`
	Pods_count            int                     `json:"pods_count,omitempty"`
	Confidence_level      float64                 `json:"confidence_level,omitempty"`
	Config                ConfigObject            `json:"config,omitempty"`
	Variation             ConfigObject            `json:"variation,omitempty"`
	Notifications         map[string]notification `json:"notifications,omitempty"`
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

func convertMetricToString(data interface{}) string {
	if metric, ok := data.(float64); ok {
		return fmt.Sprintf("%.2f", metric)
	} else {
		return ""
	}
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
			"format": "bytes",
		},
		// Memory Limit
		"memoryLimit": {
			"sum":    "memory_limit_container_sum_SUM",
			"avg":    "memory_limit_container_avg_MEAN",
			"format": "bytes",
		},
		// Memory Usage
		"memoryUsage": {
			"sum":    "memory_usage_container_sum_SUM",
			"avg":    "memory_usage_container_avg_MEAN",
			"min":    "memory_usage_container_min_MIN",
			"max":    "memory_usage_container_max_MAX",
			"format": "bytes",
		},
		// Memory RSS
		"memoryRSS": {
			"sum":    "memory_rss_usage_container_sum_SUM",
			"avg":    "memory_rss_usage_container_avg_MEAN",
			"min":    "memory_rss_usage_container_min_MIN",
			"max":    "memory_rss_usage_container_max_MAX",
			"format": "bytes",
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
			// Assign the sum value returned
			sum = convertMetricToString(c[sum_field])
		} else {
			// Set "sum" to empty string (to get it skipped in json)
			sum = ""
		}

		// Check if "avg" key exists in map
		if avg_field, ok := metricFields["avg"]; ok {
			// Assign the avg value returned
			avg = convertMetricToString(c[avg_field])
		} else {
			// Set "avg" to empty string (to get it skipped in json)
			avg = ""
		}

		// Check if "min" key exists in map
		if min_field, ok := metricFields["min"]; ok {
			// Assign the min value returned
			min = convertMetricToString(c[min_field])
		} else {
			// Set "min" to empty string (to get it skipped in json)
			min = ""
		}

		// Check if "max" key exists in map
		if max_field, ok := metricFields["max"]; ok {
			// Assign the max value returned
			max = convertMetricToString(c[max_field])
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
