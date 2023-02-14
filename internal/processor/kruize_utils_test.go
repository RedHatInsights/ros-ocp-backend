package processor

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/go-gota/gota/dataframe"
	"github.com/google/go-cmp/cmp"
)

var df dataframe.DataFrame

func TestMain(m *testing.M) {
	data := [][]string{
		{"container_name", "deployment_name", "image_name", "namespace", "cpu_request_sum_container"},
		{"ipfs-container", "ipfs", "quay.io/kubo/kubo", "ipfs-stage", "1"},
		{"ros-container", "ros", "quay.io/cloudservices/ros-backend", "ros-stage", "24"},
	}
	df = dataframe.LoadRecords(data)

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestGetAllNamespaces(t *testing.T) {
	result := get_all_namespaces(df)
	expected_result := []string{"ipfs-stage", "ros-stage"}
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestGetAllDeploymentsFromNamespace(t *testing.T) {
	result := get_all_deployments_from_namespace(df, "ros-stage")
	expected_result := []string{"ros"}
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestGetAllContainersAndImagesFromDeployment(t *testing.T) {
	result := get_all_containers_and_images_from_deployment(df, "ros-stage", "ros")
	expected_result := convert2DarrayToMap([][]string{
		{"image_name", "container_name"},
		{"quay.io/cloudservices/ros-backend", "ros-container"},
	})
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestGetAllContainersAndMetrics(t *testing.T) {
	result := get_all_containers_and_metrics(df, "ros-stage", "ros")
	expected_result := convert2DarrayToMap([][]string{
		{"image_name", "container_name", "deployment_name", "namespace", "cpu_request_sum_container"},
		{"quay.io/cloudservices/ros-backend", "ros-container", "ros", "ros-stage", "24"},
	})
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestMakeContainerData(t *testing.T) {
	container := map[string]interface{}{
		"image_name":                  "quay.io/cloudservices/ros-backend",
		"container_name":              "ros-container",
		"cpu_request_sum_container":   "1",
		"cpu_request_avg_container":   "2",
		"cpu_limit_sum_container":     "3",
		"cpu_limit_avg_container":     "4",
		"cpu_usage_sum_container":     "5",
		"cpu_usage_min_container":     "6",
		"cpu_usage_max_container":     "7",
		"cpu_usage_avg_container":     "8",
		"cpu_throttle_sum_container":  "9",
		"cpu_throttle_max_container":  "10",
		"cpu_throttle_avg_container":  "11",
		"mem_request_sum_container":   "12",
		"mem_request_avg_container":   "13",
		"mem_limit_sum_container":     "14",
		"mem_limit_avg_container":     "15",
		"mem_usage_min_container":     "16",
		"mem_usage_max_container":     "17",
		"mem_usage_sum_container":     "18",
		"mem_usage_avg_container":     "19",
		"mem-rss_usage_min_container": "20",
		"mem-rss_usage_max_container": "21",
		"mem-rss_usage_sum_container": "22",
		"mem-rss_usage_avg_container": "23",
	}

	result := make_container_data(container)
	data := `{
		"image_name": "quay.io/cloudservices/ros-backend",
		"container_name": "ros-container",
		"container_metrics": {
		  "cpuRequest" : {
			"results": {
			  "aggregation_info": {
				"sum": "1",
				"avg": "2",
				"units": "cores"
			  }
			}
		  },
		  "cpuLimit": {
			"results": {
			  "aggregation_info": {
				"sum": "3",
				"avg": "4",
				"units": "cores"
			  }
			}
		  },
		  "cpuUsage": {
			"results": {
			  "aggregation_info": {
				"min": "6",
				"max": "7",
				"sum": "5",
				"avg": "8",
				"units": "cores"
			  }
			}
		  },
		  "cpuThrottle": {
			"results": {
			  "aggregation_info": {
				"sum": "9",
				"max": "10",
				"avg": "11",
				"units": "cores"
			  }
			}
		  },
		  "memoryRequest": {
			"results": {
			  "aggregation_info": {
				"sum": "12",
				"avg": "13",
				"units": "MiB"
			  }
			}
		  },
		  "memoryLimit": {
			"results": {
			  "aggregation_info": {
				"sum": "14",
				"avg": "15",
				"units": "MiB"
			  }
			}
		  },
		  "memoryUsage": {
			"results": {
			  "aggregation_info": {
				"min": "16",
				"max": "17",
				"sum": "18",
				"avg": "19",
				"units": "MiB"
			  }
			}
		  },
		  "memoryRSS": {
			"results": {
			  "aggregation_info": {
				"min": "20",
				"max": "21",
				"sum": "22",
				"avg": "23",
				"units": "MiB"
			  }
			}
		  }
		}
	  }`

	expected_result := make(map[string]interface{})
	json.Unmarshal([]byte(data), &expected_result)
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}

}
