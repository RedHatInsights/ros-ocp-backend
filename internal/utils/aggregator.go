package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	w "github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
)

func Aggregate_data(df dataframe.DataFrame) (dataframe.DataFrame, error) {
	log = logging.GetLogger()

	// Check if CSV has all the needed columns
	if err := check_if_all_required_columns_in_CSV(df); err != nil {
		return dataframe.DataFrame{}, err
	}

	// Validation to check if metrics for cpuUsage, memoryUsage and memoryRSS are missing
	df, no_of_dropped_records := filter_valid_csv_records(df)
	if no_of_dropped_records != 0 {
		invalidDataPoints.Add(float64(no_of_dropped_records))
		log.Infof("Invalid records in CSV - %v", no_of_dropped_records)
	}

	if df.Nrow() == 0 {
		log.Error("No valid records present in CSV to process further.")
		return df, nil
	}

	df = determine_k8s_object_type(df)

	// filter out only valid workload type
	df = filter_valid_k8s_object_types(df)

	dfGroups := df.GroupBy(
		"namespace",
		"k8s_object_type",
		"k8s_object_name",
		"workload",
		"container_name",
		"image_name",
		"interval_start",
		"interval_end",
	)

	aggregationMapping := map[string]dataframe.AggregationType{
		"cpu_request_container_avg":      dataframe.Aggregation_MEAN,
		"cpu_request_container_sum":      dataframe.Aggregation_SUM,
		"cpu_limit_container_avg":        dataframe.Aggregation_MEAN,
		"cpu_limit_container_sum":        dataframe.Aggregation_SUM,
		"cpu_usage_container_avg":        dataframe.Aggregation_MEAN,
		"cpu_usage_container_min":        dataframe.Aggregation_MIN,
		"cpu_usage_container_max":        dataframe.Aggregation_MAX,
		"cpu_usage_container_sum":        dataframe.Aggregation_SUM,
		"cpu_throttle_container_avg":     dataframe.Aggregation_MEAN,
		"cpu_throttle_container_max":     dataframe.Aggregation_MAX,
		"cpu_throttle_container_sum":     dataframe.Aggregation_SUM,
		"memory_request_container_avg":   dataframe.Aggregation_MEAN,
		"memory_request_container_sum":   dataframe.Aggregation_SUM,
		"memory_limit_container_avg":     dataframe.Aggregation_MEAN,
		"memory_limit_container_sum":     dataframe.Aggregation_SUM,
		"memory_usage_container_avg":     dataframe.Aggregation_MEAN,
		"memory_usage_container_min":     dataframe.Aggregation_MIN,
		"memory_usage_container_max":     dataframe.Aggregation_MAX,
		"memory_usage_container_sum":     dataframe.Aggregation_SUM,
		"memory_rss_usage_container_avg": dataframe.Aggregation_MEAN,
		"memory_rss_usage_container_min": dataframe.Aggregation_MIN,
		"memory_rss_usage_container_max": dataframe.Aggregation_MAX,
		"memory_rss_usage_container_sum": dataframe.Aggregation_SUM,
	}

	columnsToAggregate := []string{}
	columnsAggregationType := []dataframe.AggregationType{}
	for k, v := range aggregationMapping {
		columnsToAggregate = append(columnsToAggregate, k)
		columnsAggregationType = append(columnsAggregationType, v)
	}

	df = dfGroups.Aggregation(columnsAggregationType, columnsToAggregate)
	return df, nil
}

func filter_valid_csv_records(main_df dataframe.DataFrame) (dataframe.DataFrame, int) {
	df := main_df.FilterAggregation(
		dataframe.And,
		dataframe.F{Colname: "memory_rss_usage_container_sum", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_rss_usage_container_max", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_rss_usage_container_min", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_rss_usage_container_avg", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_usage_container_sum", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_usage_container_max", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_usage_container_min", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "memory_usage_container_avg", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "cpu_usage_container_sum", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "cpu_usage_container_max", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "cpu_usage_container_min", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "cpu_usage_container_avg", Comparator: series.GreaterEq, Comparando: 0},
		dataframe.F{Colname: "owner_kind", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "owner_name", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "workload", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "workload_type", Comparator: series.Neq, Comparando: ""},
	)

	no_of_dropped_records := main_df.Nrow() - df.Nrow()

	return df, no_of_dropped_records
}

func filter_valid_k8s_object_types(df dataframe.DataFrame) dataframe.DataFrame {
	return df.Filter(
		dataframe.F{
			Colname:    "k8s_object_type",
			Comparator: series.In,
			Comparando: []string{
				w.Daemonset.String(),
				w.Deployment.String(),
				w.Deploymentconfig.String(),
				w.Replicaset.String(),
				w.Replicationcontroller.String(),
				w.Statefulset.String(),
			}},
	)
}

func determine_k8s_object_type(df dataframe.DataFrame) dataframe.DataFrame {
	columns := df.Names()
	index_of_owner_name := findInStringSlice("owner_name", columns)
	index_of_owner_kind := findInStringSlice("owner_kind", columns)
	index_of_workload := findInStringSlice("workload", columns)
	index_of_workload_type := findInStringSlice("workload_type", columns)

	s := df.Rapply(func(s series.Series) series.Series {
		owner_name := s.Elem(index_of_owner_name).String()
		owner_kind := s.Elem(index_of_owner_kind).String()
		workload := s.Elem(index_of_workload).String()
		workload_type := s.Elem(index_of_workload_type).String()
		if strings.ToLower(owner_kind) == string(w.Replicaset) && workload == "<none>" {
			return series.Strings([]string{string(w.Replicaset), owner_name})
		} else if strings.ToLower(owner_kind) == string(w.Replicationcontroller) && workload == "<none>" {
			return series.Strings([]string{string(w.Replicationcontroller), owner_name})
		} else {
			return series.Strings([]string{workload_type, workload})
		}
	})

	df = df.Mutate(s.Col("X0")).Rename("k8s_object_type", "X0")
	df = df.Mutate(s.Col("X1")).Rename("k8s_object_name", "X1")
	return df
}

func check_if_all_required_columns_in_CSV(df dataframe.DataFrame) error {
	// Check if all the required columns are present in CSV
	all_required_columns := []string{
		"report_period_start",
		"report_period_end",
		"interval_start",
		"interval_end",
		"container_name",
		"pod",
		"owner_name",
		"owner_kind",
		"workload",
		"workload_type",
		"namespace",
		"image_name",
		"node",
		"resource_id",
		"cpu_request_container_avg",
		"cpu_request_container_sum",
		"cpu_limit_container_avg",
		"cpu_limit_container_sum",
		"cpu_usage_container_avg",
		"cpu_usage_container_min",
		"cpu_usage_container_max",
		"cpu_usage_container_sum",
		"cpu_throttle_container_avg",
		"cpu_throttle_container_max",
		"cpu_throttle_container_sum",
		"memory_request_container_avg",
		"memory_request_container_sum",
		"memory_limit_container_avg",
		"memory_limit_container_sum",
		"memory_usage_container_avg",
		"memory_usage_container_min",
		"memory_usage_container_max",
		"memory_usage_container_sum",
		"memory_rss_usage_container_avg",
		"memory_rss_usage_container_min",
		"memory_rss_usage_container_max",
		"memory_rss_usage_container_sum",
	}
	cloumns_in_csv := df.Names()
	if !reflect.DeepEqual(all_required_columns, cloumns_in_csv) {
		return fmt.Errorf("CSV file does not have all the required columns")
	}
	return nil
}
