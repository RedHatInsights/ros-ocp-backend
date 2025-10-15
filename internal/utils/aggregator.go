package utils

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
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
		return df, fmt.Errorf("no valid records present in CSV to process further")
	}
	df = determine_k8s_object_type(df)

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

func filter_valid_csv_records(mainDf dataframe.DataFrame) (dataframe.DataFrame, int) {
	df := mainDf.FilterAggregation(
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
		dataframe.F{Colname: "owner_kind", Comparator: series.Neq, Comparando: "<none>"},
		dataframe.F{Colname: "owner_name", Comparator: series.Neq, Comparando: "<none>"},
		dataframe.F{Colname: "workload_type", Comparator: series.Neq, Comparando: "<none>"},
		dataframe.F{Colname: "workload_type", Comparator: series.Neq, Comparando: ""},
	)

	// The above filters can delete all the rows
	if df.Nrow() == 0 {
		return df, mainDf.Nrow()
	}

	// Change the case of all workload_type to lowercase
	lcaseWorkloadTypes := df.Rapply(func(s series.Series) series.Series {
		columns := df.Names()
		indexOfWorkloadType := findInStringSlice("workload_type", columns)
		workloadType := s.Elem(indexOfWorkloadType).String()
		lcaseWorkloadType := strings.ToLower(workloadType)
		return series.Strings([]string{lcaseWorkloadType})
	})

	// Delete existing workload_type column
	df = df.Mutate(df.Col("workload_type")).Drop("workload_type")

	// Rename lowercase converted column to workload_type
	df = df.Mutate(lcaseWorkloadTypes.Col("X0")).Rename("workload_type", "X0")

	df = df.FilterAggregation(
		dataframe.And,
		dataframe.F{
			Colname:    "workload_type",
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

	noOfDroppedRecords := mainDf.Nrow() - df.Nrow()

	return df, noOfDroppedRecords
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
		if strings.ToLower(owner_kind) == string(w.Replicaset) && (workload == "<none>" || workload == "") {
			return series.Strings([]string{string(w.Replicaset), owner_name})
		} else if strings.ToLower(owner_kind) == string(w.Replicationcontroller) && (workload == "<none>" || workload == "") {
			return series.Strings([]string{string(w.Replicationcontroller), owner_name})
		} else {
			return series.Strings([]string{workload_type, workload})
		}
	})

	df = df.Mutate(s.Col("X0")).Rename("k8s_object_type", "X0")
	df = df.Mutate(s.Col("X1")).Rename("k8s_object_name", "X1")
	return df
}

func elementsMatch(requiredColumns []string, csvColumns []string) bool {
	for _, reqCol := range requiredColumns {
		if !slices.Contains(csvColumns, reqCol) {
			log.Warnf("missing columns in CSV: %v", reqCol)
			return false
		}
	}
	return true
}

func check_if_all_required_columns_in_CSV(df dataframe.DataFrame) error {
	// Check if all the required columns are present in CSV
	all_required_columns := make([]string, 0, len(types.CSVColumnMapping))
	for k := range types.CSVColumnMapping {
		all_required_columns = append(all_required_columns, k)
	}

	columns_in_csv := df.Names()
	if !elementsMatch(all_required_columns, columns_in_csv) {
		return fmt.Errorf("CSV file does not have all the required columns")
	}
	return nil
}
