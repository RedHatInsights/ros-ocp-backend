package processor

import (
	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
)

func get_all_namespaces(df dataframe.DataFrame) []string {
	namespaces := df.Select([]string{"namespace"}).Col("namespace")
	return unique(namespaces.Records())
}

func get_all_deployments_from_namespace(df dataframe.DataFrame, namespace string) []string {
	fil := df.Filter(
		dataframe.F{
			Colname:    "namespace",
			Comparator: series.Eq,
			Comparando: namespace,
		},
	)
	deployments := fil.Select([]string{"deployment_name"}).Col("deployment_name")
	return unique(deployments.Records())
}

func get_all_containers_and_images_from_deployment(df dataframe.DataFrame, namespace, deployment_name string) []map[string]interface{} {
	fil := df.FilterAggregation(
		dataframe.And,
		dataframe.F{
			Colname:    "namespace",
			Comparator: series.Eq,
			Comparando: namespace,
		},
		dataframe.F{
			Colname:    "deployment_name",
			Comparator: series.Eq,
			Comparando: deployment_name,
		},
	)
	c := fil.Select([]string{"container_name", "image_name"}).Records()
	data := convert2DarrayToMap(c)
	return data

}

func get_all_containers_and_metrics(df dataframe.DataFrame, namespace, deployment_name string) []map[string]interface{} {
	fil := df.FilterAggregation(
		dataframe.And,
		dataframe.F{
			Colname:    "namespace",
			Comparator: series.Eq,
			Comparando: namespace,
		},
		dataframe.F{
			Colname:    "deployment_name",
			Comparator: series.Eq,
			Comparando: deployment_name,
		},
	)

	data := convert2DarrayToMap(fil.Records())
	return data
}

func make_container_data(container map[string]interface{}) map[string]interface{} {
	container_data := make(map[string]interface{})
	container_data["image_name"] = container["image_name"]
	container_data["container_name"] = container["container_name"]
	container_data["container_metrics"] = make(map[string]string)

	container_data["container_metrics"] = map[string]interface{}{

		"cpuRequest": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["cpu_request_sum_container"],
					"avg":   container["cpu_request_avg_container"],
					"units": "cores",
				},
			},
		},

		"cpuLimit": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["cpu_limit_sum_container"],
					"avg":   container["cpu_limit_avg_container"],
					"units": "cores",
				},
			},
		},

		"cpuUsage": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["cpu_usage_sum_container"],
					"min":   container["cpu_usage_min_container"],
					"max":   container["cpu_usage_max_container"],
					"avg":   container["cpu_usage_avg_container"],
					"units": "cores",
				},
			},
		},

		"cpuThrottle": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["cpu_throttle_sum_container"],
					"max":   container["cpu_throttle_max_container"],
					"avg":   container["cpu_throttle_avg_container"],
					"units": "cores",
				},
			},
		},

		"memoryRequest": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["mem_request_sum_container"],
					"avg":   container["mem_request_avg_container"],
					"units": "MiB",
				},
			},
		},

		"memoryLimit": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"sum":   container["mem_limit_sum_container"],
					"avg":   container["mem_limit_avg_container"],
					"units": "MiB",
				},
			},
		},

		"memoryUsage": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"min":   container["mem_usage_min_container"],
					"max":   container["mem_usage_max_container"],
					"sum":   container["mem_usage_sum_container"],
					"avg":   container["mem_usage_avg_container"],
					"units": "MiB",
				},
			},
		},

		"memoryRSS": map[string]interface{}{
			"results": map[string]interface{}{
				"aggregation_info": map[string]interface{}{
					"min":   container["mem-rss_usage_min_container"],
					"max":   container["mem-rss_usage_max_container"],
					"sum":   container["mem-rss_usage_sum_container"],
					"avg":   container["mem-rss_usage_avg_container"],
					"units": "MiB",
				},
			},
		},
	}

	return container_data

}
