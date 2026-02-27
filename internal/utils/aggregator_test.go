package utils

import (
	"testing"

	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
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

func Test_filter_valid_csv_records(t *testing.T) {
	csvTypeContainer := types.PayloadTypeContainer
	usage_data := []UsageData{
		// k8s object with missing data
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "",
		},
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "", "", "", "", "1", "1", "1", "1", "1", "1", "1", "", "", "", "", "", "", "", "",
		},
		// k8s object with 0 CPU, Memory and RSS usage
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "0", "0", "0", "0", "1", "1", "1", "1", "1", "1", "1", "0", "0", "0", "0", "0", "0", "0", "0",
		},
	}
	df := dataframe.LoadStructs(usage_data)
	result, no_of_dropped_records := filterValidCSVRecords(csvTypeContainer, df)
	if result.Nrow() != 1 || no_of_dropped_records != 2 {
		t.Error("Invalid k8s object type did not get dropped")
	}

	usage_data = []UsageData{
		// k8s object type DaemonSet
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "DaemonSet", "testdeploymentconfig", "daemonset", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		// k8s object type Replicaset
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "<none>", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		// k8s object type Deployment
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "deployment", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		// k8s object type ReplicationController
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicationController", "<none>", "deploymentconfig", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		// k8s object type Deploymentconfig
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicationController", "testdeploymentconfig", "deploymentconfig", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		// k8s object type StatefulSet
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "StatefulSet", "testdeploymentconfig", "statefulset", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result, _ = filterValidCSVRecords(csvTypeContainer, df)
	if result.Nrow() != 6 {
		t.Error("Data not filtered properly. Some of the valid k8s object type got dropped")
	}

	// check if Invalid k8s object type is dropped
	usage_data = []UsageData{
		// k8s object type Job
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "Job", "testdeploymentconfig", "job", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result, _ = filterValidCSVRecords(csvTypeContainer, df)
	if result.Nrow() != 0 {
		t.Error("Invalid k8s object type did not get dropped")
	}

	// check if empty workload_type is dropped
	usage_data = []UsageData{
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
		{
			"2023-02-01 00:00:00 +0000 UTC", "2023-03-01 00:00:00 +0000 UTC", "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC",
			"Yuptoo-service", "Yuptoo-app-standalone-1", "Yuptoo-app", "ReplicaSet", "testdeployment", "<none>", "Yuptoo-prod",
			"quay.io/cloudservices/yuptoo", "ip-10-0-176-227.us-east-2.compute.internal", "i-0dfbb3fa4d0e8fc94",
			"1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1", "1",
		},
	}
	df = dataframe.LoadStructs(usage_data)
	result, _ = filterValidCSVRecords(csvTypeContainer, df)
	if result.Nrow() != 0 {
		t.Error("Invalid k8s object type did not get dropped")
	}
}

func Test_check_if_all_required_columns_in_CSV(t *testing.T) {
	// Good case - all the columns are present
	usage_data := []UsageData{{}}
	df := dataframe.LoadStructs(usage_data)
	if err := hasMissingColumnsCSV(types.PayloadTypeContainer, df); err != nil {
		t.Error("CSV has all required columns but test fails")
	}

	// Should allow change in column order.
	columns := df.Names()
	columns[1], columns[2] = columns[2], columns[1]
	newdf := dataframe.LoadRecords(
		[][]string{
			columns,
			columns,
		},
	)
	if err := hasMissingColumnsCSV(types.PayloadTypeContainer, newdf); err != nil {
		t.Error("unordered columns should be allowed")
	}

	// Bad case - dropping one of the column
	df = df.Drop([]int{5})
	if err := hasMissingColumnsCSV(types.PayloadTypeContainer, df); err == nil {
		t.Error("Expecting error to be returned as all required column not present")
	}

	// Case for covering additional columns in CSV
	usageData := []UsageData{{}}
	df = dataframe.LoadStructs(usageData)
	df = df.Mutate(
		series.New([]string{"abc"}, series.String, "additional_column_1"),
	)
	df = df.Mutate(
		series.New([]string{"abc_profile"}, series.String, "additional_column_2"),
	)

	if err := hasMissingColumnsCSV(types.PayloadTypeContainer, df); err != nil {
		t.Error("additional columns should be ignored but test fails")
	}
}

func TestAggregateDataNoRecords(t *testing.T) {
	usage_data := []UsageData{}

	// The function should not panic when none of the rows are valid
	df := dataframe.LoadStructs(usage_data)
	_, err := Aggregate_data(types.PayloadTypeContainer, df)
	if err == nil {
		t.Error("Expecting error to be returned when all rows are invalid")
	}
}

type NamespaceUsageData struct {
	Report_period_start            string `dataframe:"report_period_start,string"`
	Report_period_end              string `dataframe:"report_period_end,string"`
	Interval_start                 string `dataframe:"interval_start,string"`
	Interval_end                   string `dataframe:"interval_end,string"`
	Namespace                      string `dataframe:"namespace,string"`
	Cpu_request_namespace_sum      string `dataframe:"cpu_request_namespace_sum,float"`
	Cpu_limit_namespace_sum        string `dataframe:"cpu_limit_namespace_sum,float"`
	Cpu_usage_namespace_avg        string `dataframe:"cpu_usage_namespace_avg,float"`
	Cpu_usage_namespace_max        string `dataframe:"cpu_usage_namespace_max,float"`
	Cpu_usage_namespace_min        string `dataframe:"cpu_usage_namespace_min,float"`
	Cpu_throttle_namespace_avg     string `dataframe:"cpu_throttle_namespace_avg,float"`
	Cpu_throttle_namespace_max     string `dataframe:"cpu_throttle_namespace_max,float"`
	Cpu_throttle_namespace_min     string `dataframe:"cpu_throttle_namespace_min,float"`
	Memory_request_namespace_sum   string `dataframe:"memory_request_namespace_sum,float"`
	Memory_limit_namespace_sum     string `dataframe:"memory_limit_namespace_sum,float"`
	Memory_usage_namespace_avg     string `dataframe:"memory_usage_namespace_avg,float"`
	Memory_usage_namespace_max     string `dataframe:"memory_usage_namespace_max,float"`
	Memory_usage_namespace_min     string `dataframe:"memory_usage_namespace_min,float"`
	Memory_rss_usage_namespace_avg string `dataframe:"memory_rss_usage_namespace_avg,float"`
	Memory_rss_usage_namespace_max string `dataframe:"memory_rss_usage_namespace_max,float"`
	Memory_rss_usage_namespace_min string `dataframe:"memory_rss_usage_namespace_min,float"`
	Namespace_running_pods_max     string `dataframe:"namespace_running_pods_max,float"`
	Namespace_running_pods_avg     string `dataframe:"namespace_running_pods_avg,float"`
	Namespace_total_pods_max       string `dataframe:"namespace_total_pods_max,float"`
	Namespace_total_pods_avg       string `dataframe:"namespace_total_pods_avg,float"`
}

func TestFilterValidCSVRecordsNamespace(t *testing.T) {
	t0, t1 := "2023-06-02 00:00:01 +0000 UTC", "2023-06-02 00:15:00 +0000 UTC"
	e := ""
	nsType := types.PayloadTypeNamespace

	// invalid records (missing metrics); should be dropped
	df := dataframe.LoadStructs([]NamespaceUsageData{
		{t0, t1, t0, t1, "test-ns", e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e},
		{t0, t1, t0, t1, "test-ns", e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e, e},
		{t0, t1, t0, t1, "test-ns", e, e, "0", "0", "0", e, e, e, e, e, "0", "0", "0", "0", "0", "0", "1", "1", "1", "1"},
	})
	result, dropped := filterValidCSVRecords(nsType, df)
	if result.Nrow() != 1 || dropped != 2 {
		t.Error("invalid namespace records did not get dropped")
	}

	// valid set of records
	df = dataframe.LoadStructs([]NamespaceUsageData{
		{t0, t1, t0, t1, "test-ns-1", e, e, "0.5", "1.0", "0.1", e, e, e, e, e, "1000000", "2000000", "500000", "800000", "1000000", "600000", "3", "3", "3", "3"},
		{t0, t1, t0, t1, "test-ns-2", e, e, "0.3", "0.8", "0.05", e, e, e, e, e, "800000", "1500000", "400000", "700000", "900000", "500000", "2", "2", "2", "2"},
	})
	result, _ = filterValidCSVRecords(nsType, df)
	if result.Nrow() != 2 {
		t.Error("valid namespace records got dropped")
	}

	// empty, <none> namespaces should be dropped
	df = dataframe.LoadStructs([]NamespaceUsageData{
		{t0, t1, t0, t1, "", e, e, "0.5", "1.0", "0.1", e, e, e, e, e, "1000000", "2000000", "500000", "800000", "1000000", "600000", "3", "3", "3", "3"},
		{t0, t1, t0, t1, "<none>", e, e, "0.3", "0.8", "0.05", e, e, e, e, e, "800000", "1500000", "400000", "700000", "900000", "500000", "2", "2", "2", "2"},
	})
	result, _ = filterValidCSVRecords(nsType, df)
	if result.Nrow() != 0 {
		t.Error("invalid (empty or <none>) namespace record did not get dropped")
	}

	// negative value records should be dropped
	df = dataframe.LoadStructs([]NamespaceUsageData{
		{t0, t1, t0, t1, "test-ns", e, e, "-0.5", "1.0", "0.1", e, e, e, e, e, "1000000", "2000000", "500000", "800000", "1000000", "600000", "3", "3", "3", "3"},
	})
	result, _ = filterValidCSVRecords(nsType, df)
	if result.Nrow() != 0 {
		t.Error("invalid namespace record with negative CPU usage did not get dropped")
	}
}

func TestRequiredColumnsNamespaceCSV(t *testing.T) {
	nsType := types.PayloadTypeNamespace
	df := dataframe.LoadStructs([]NamespaceUsageData{{}})

	if err := hasMissingColumnsCSV(nsType, df); err != nil {
		t.Error("csv has all required columns but test fails")
	}

	cols := df.Names()
	if len(cols) > 2 {
		cols[1], cols[2] = cols[2], cols[1]
		newdf := dataframe.LoadRecords([][]string{cols, cols})
		if err := hasMissingColumnsCSV(nsType, newdf); err != nil {
			t.Error("unordered columns should be allowed")
		}
	}

	df = df.Drop([]int{5})
	if err := hasMissingColumnsCSV(nsType, df); err == nil {
		t.Error("expecting error when required column is missing")
	}

	df = dataframe.LoadStructs([]NamespaceUsageData{{}})
	df = df.Mutate(series.New([]string{"abc"}, series.String, "extra_col_1"))
	df = df.Mutate(series.New([]string{"def"}, series.String, "extra_col_2"))
	if err := hasMissingColumnsCSV(nsType, df); err != nil {
		t.Error("additional columns should be ignored")
	}
}

func TestAggregateDataNoRecordsNamespace(t *testing.T) {
	df := dataframe.LoadStructs([]NamespaceUsageData{})
	_, err := Aggregate_data(types.PayloadTypeNamespace, df)
	if err == nil {
		t.Error("expecting error when no records present")
	}
}
