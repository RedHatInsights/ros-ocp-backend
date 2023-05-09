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
	Notifications []notification                `json:"notifications,omitempty"`
}

type notification struct {
	NotifyType string `json:"type,omitempty"`
	Message    string `json:"message,omitempty"`
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
	Monitoring_start_time time.Time       `json:"monitoring_start_time,omitempty"`
	Monitoring_end_time   time.Time       `json:"monitoring_end_time,omitempty"`
	Duration_in_hours     float64         `json:"duration_in_hours,omitempty"`
	Pods_count            int             `json:"pods_count,omitempty"`
	Confidence_level      float64         `json:"confidence_level,omitempty"`
	Config                ConfigObject    `json:"config,omitempty"`
	Variation             ConfigObject    `json:"variation,omitempty"`
	Notifications         []notifications `json:"notifications,omitempty"`
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

type notifications struct {
	Notificationtype string `json:"type,omitempty"`
	Message          string `json:"message,omitempty"`
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

	// cpuRequest
	sum := convertMetricToString(c["cpu_request_container_sum_SUM"])
	avg := convertMetricToString(c["cpu_request_container_avg_MEAN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "cpuRequest",
			Results: result{
				Aggregation_info: aggregation_info{
					Sum:    sum,
					Avg:    avg,
					Format: "cores",
				},
			},
		})
	}

	// cpuLimit
	sum = convertMetricToString(c["cpu_limit_container_sum_SUM"])
	avg = convertMetricToString(c["cpu_limit_container_avg_MEAN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "cpuLimit",
			Results: result{
				Aggregation_info: aggregation_info{
					Sum:    sum,
					Avg:    avg,
					Format: "cores",
				},
			},
		})
	}

	// cpuUsage
	sum = convertMetricToString(c["cpu_usage_container_sum_SUM"])
	avg = convertMetricToString(c["cpu_usage_container_avg_MEAN"])
	max := convertMetricToString(c["cpu_usage_container_max_MAX"])
	min := convertMetricToString(c["cpu_usage_container_min_MIN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "cpuUsage",
			Results: result{
				Aggregation_info: aggregation_info{
					Min:    min,
					Max:    max,
					Sum:    sum,
					Avg:    avg,
					Format: "cores",
				},
			},
		})
	}

	// cpuThrottle
	sum = convertMetricToString(c["cpu_throttle_container_sum_SUM"])
	avg = convertMetricToString(c["cpu_throttle_container_avg_MEAN"])
	max = convertMetricToString(c["cpu_throttle_container_max_MAX"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "cpuThrottle",
			Results: result{
				Aggregation_info: aggregation_info{
					Max:    max,
					Sum:    sum,
					Avg:    avg,
					Format: "cores",
				},
			},
		})
	}

	// memoryRequest
	sum = convertMetricToString(c["memory_request_container_sum_SUM"])
	avg = convertMetricToString(c["memory_request_container_avg_MEAN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "memoryRequest",
			Results: result{
				Aggregation_info: aggregation_info{
					Sum:    sum,
					Avg:    avg,
					Format: "MiB",
				},
			},
		})
	}

	// memoryLimit
	sum = convertMetricToString(c["memory_limit_container_sum_SUM"])
	avg = convertMetricToString(c["memory_limit_container_avg_MEAN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "memoryLimit",
			Results: result{
				Aggregation_info: aggregation_info{
					Sum:    sum,
					Avg:    avg,
					Format: "MiB",
				},
			},
		})
	}

	// memoryUsage
	sum = convertMetricToString(c["memory_usage_container_sum_SUM"])
	avg = convertMetricToString(c["memory_usage_container_avg_MEAN"])
	min = convertMetricToString(c["memory_usage_container_min_MIN"])
	max = convertMetricToString(c["memory_usage_container_max_MAX"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "memoryUsage",
			Results: result{
				Aggregation_info: aggregation_info{
					Min:    min,
					Max:    max,
					Sum:    sum,
					Avg:    avg,
					Format: "MiB",
				},
			},
		})
	}

	// memoryRSS
	sum = convertMetricToString(c["memory_rss_usage_container_sum_SUM"])
	avg = convertMetricToString(c["memory_rss_usage_container_avg_MEAN"])
	max = convertMetricToString(c["memory_rss_usage_container_min_MIN"])
	min = convertMetricToString(c["memory_rss_usage_container_min_MIN"])
	if sum != "" && avg != "" {
		metrics = append(metrics, metric{
			Name: "memoryRSS",
			Results: result{
				Aggregation_info: aggregation_info{
					Min:    min,
					Max:    max,
					Sum:    sum,
					Avg:    avg,
					Format: "MiB",
				},
			},
		})
	}

	container_data := container{
		Container_image_name: c["image_name"].(string),
		Container_name:       c["container_name"].(string),
		Metrics:              metrics,
	}

	return container_data
}
