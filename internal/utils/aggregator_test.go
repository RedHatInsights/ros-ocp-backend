package utils

import (
	"testing"

	"github.com/go-gota/gota/dataframe"
	"github.com/google/go-cmp/cmp"
)

type UsageData struct {
	Report_period_start            string `dataframe:"report_period_start,string"`
	Report_period_end              string `dataframe:"report_period_end,string"`
	Interval_start                 string `dataframe:"interval_start,string"`
	Interval_end                   string `dataframe:"interval_end,string"`
	Container_name                 string `dataframe:"container_name,string"`
	Pod                            string `dataframe:"pod,string"`
	Owner_name                     string `dataframe:"owner_name,string"`
	Owner_kind                     string `dataframe:"owner_kind,string"`
	Workload                       string `dataframe:"workload,string"`
	Workload_type                  string `dataframe:"workload_type,string"`
	Namespace                      string `dataframe:"namespace,string"`
	Image_name                     string `dataframe:"image_name,string"`
	Node                           string `dataframe:"node,string"`
	Resource_id                    string `dataframe:"resource_id,string"`
	Cpu_request_container_avg      string `dataframe:"cpu_request_container_avg,float"`
	Cpu_request_container_sum      string `dataframe:"cpu_request_container_sum,float"`
	Cpu_limit_container_avg        string `dataframe:"cpu_limit_container_avg,float"`
	Cpu_limit_container_sum        string `dataframe:"cpu_limit_container_sum,float"`
	Cpu_usage_container_avg        string `dataframe:"cpu_usage_container_avg,float"`
	Cpu_usage_container_min        string `dataframe:"cpu_usage_container_min,float"`
	Cpu_usage_container_max        string `dataframe:"cpu_usage_container_max,float"`
	Cpu_usage_container_sum        string `dataframe:"cpu_usage_container_sum,float"`
	Cpu_throttle_container_avg     string `dataframe:"cpu_throttle_container_avg,float"`
	Cpu_throttle_container_max     string `dataframe:"cpu_throttle_container_max,float"`
	Cpu_throttle_container_sum     string `dataframe:"cpu_throttle_container_sum,float"`
	Memory_request_container_avg   string `dataframe:"memory_request_container_avg,float"`
	Memory_request_container_sum   string `dataframe:"memory_request_container_sum,float"`
	Memory_limit_container_avg     string `dataframe:"memory_limit_container_avg,float"`
	Memory_limit_container_sum     string `dataframe:"memory_limit_container_sum,float"`
	Memory_usage_container_avg     string `dataframe:"memory_usage_container_avg,float"`
	Memory_usage_container_min     string `dataframe:"memory_usage_container_min,float"`
	Memory_usage_container_max     string `dataframe:"memory_usage_container_max,float"`
	Memory_usage_container_sum     string `dataframe:"memory_usage_container_sum,float"`
	Memory_rss_usage_container_avg string `dataframe:"memory_rss_usage_container_avg,float"`
	Memory_rss_usage_container_min string `dataframe:"memory_rss_usage_container_min,float"`
	Memory_rss_usage_container_max string `dataframe:"memory_rss_usage_container_max,float"`
	Memory_rss_usage_container_sum string `dataframe:"memory_rss_usage_container_sum,float"`
}

// Test to check if aggregation function clean data properly.
func TestAggregateCleaningData(t *testing.T) {

	// Case if owner_name is absent
	usage_data := []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df := dataframe.LoadStructs(usage_data)
	result := Aggregate_data(df)

	if result.Nrow() > 0 {
		t.Errorf("Aggregator function hasn't cleaned the data properly. (case - owner_name) Expected - 0 but got %v", result.Nrow())
	}

	// Case if owner_kind is absent
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "", "<none>", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result = Aggregate_data(df)

	if result.Nrow() > 0 {
		t.Errorf("Aggregator function hasn't clean the data properly. (case - owner_kind) Expected - 0 but got %v", result.Nrow())
	}

	// Case if workload is absent
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result = Aggregate_data(df)

	if result.Nrow() > 0 {
		t.Errorf("Aggregator function hasn't clean the data properly. (case - workload) Expected - 0 but got %v", result.Nrow())
	}

	// Case if workload_type is absent
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "<none>", "", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result = Aggregate_data(df)

	if result.Nrow() > 0 {
		t.Errorf("Aggregator function hasn't clean the data properly. (case - workload_type) Expected - 0 but got %v", result.Nrow())
	}

	// Case if all columns are absent
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "", "", "", "", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result = Aggregate_data(df)

	if result.Nrow() > 0 {
		t.Errorf("Aggregator function hasn't clean the data properly. (case - all columns absent) Expected - 0 but got %v", result.Nrow())
	}

	// Negative Case - don't delete if all columns are present
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result = Aggregate_data(df)

	if result.Nrow() == 0 {
		t.Errorf("Aggregator function hasn't clean the data properly. (case - all columns present) Expected - 1 but got %v", result.Nrow())
	}
}

func TestAggregateAddingNewColumn(t *testing.T) {
	usage_data := []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df := dataframe.LoadStructs(usage_data)
	result := Aggregate_data(df)

	if result.Nrow() != 1 {
		t.Errorf("Expected number of row - 1 but got %v", result.Nrow())
	} else {
		data := result.Maps()[0]
		if data["k8s_object_type"] != "replicaset" || data["k8s_object_name"] != "Yuptoo-app" {
			t.Errorf("Expected k8s_object_type = replicaset and k8s_object_name = Yuptoo-app but got k8s_object_type = %s and k8s_object_name = %s", data["k8s_object_type"], data["k8s_object_name"])
		}
	}

}

func TestAggregateData(t *testing.T) {
	usage_data := []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod", "quay.io/cloudservices/yuptoo",
			"ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-04-02 00:00:01 +0000 UTC", "2023-04-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-2", "Yuptoo-app", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod", "quay.io/cloudservices/yuptoo",
			"ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2", "2",
		},
	}

	df := dataframe.LoadStructs(usage_data)
	result := Aggregate_data(df)
	if result.Nrow() != 1 {
		t.Errorf("Expected number of row - 1 but got %v", result.Nrow())
	} else {
		data := result.Maps()[0]
		expected_data := map[string]interface{}{
			"container_name":                      "Yuptoo-service",
			"cpu_limit_container_avg_MEAN":        1.5,
			"cpu_limit_container_sum_SUM":         3.0,
			"cpu_request_container_avg_MEAN":      1.5,
			"cpu_request_container_sum_SUM":       3.0,
			"cpu_throttle_container_avg_MEAN":     1.5,
			"cpu_throttle_container_max_MAX":      2.0,
			"cpu_throttle_container_sum_SUM":      3.0,
			"cpu_usage_container_avg_MEAN":        1.5,
			"cpu_usage_container_max_MAX":         2.0,
			"cpu_usage_container_min_MIN":         1.0,
			"cpu_usage_container_sum_SUM":         3.0,
			"image_name":                          "quay.io/cloudservices/yuptoo",
			"interval_end":                        "2023-04-02 00:15:00 +0000 UTC",
			"interval_start":                      "2023-04-02 00:00:01 +0000 UTC",
			"k8s_object_name":                     "Yuptoo-app",
			"k8s_object_type":                     "replicaset",
			"memory_limit_container_avg_MEAN":     1.5,
			"memory_limit_container_sum_SUM":      3.0,
			"memory_request_container_avg_MEAN":   1.5,
			"memory_request_container_sum_SUM":    3.0,
			"memory_rss_usage_container_avg_MEAN": 1.5,
			"memory_rss_usage_container_max_MAX":  2.0,
			"memory_rss_usage_container_min_MIN":  1.0,
			"memory_rss_usage_container_sum_SUM":  3.0,
			"memory_usage_container_avg_MEAN":     1.5,
			"memory_usage_container_max_MAX":      2.0,
			"memory_usage_container_min_MIN":      1.0,
			"memory_usage_container_sum_SUM":      3.0,
			"namespace":                           "Yuptoo-prod",
			"workload":                            "<none>",
		}
		if diff := cmp.Diff(data, expected_data); diff != "" {
			t.Error(diff)
		}
	}
}
