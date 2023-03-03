package processor

import (
	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
)

func Aggregate_data(df dataframe.DataFrame) dataframe.DataFrame {
	df = df.Filter(
		dataframe.F{Colname: "owner_kind", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "owner_name", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "workload", Comparator: series.Neq, Comparando: ""},
		dataframe.F{Colname: "workload_type", Comparator: series.Neq, Comparando: ""},
	)

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
		if owner_kind == "ReplicaSet" && workload == "<none>" {
			return series.Strings([]string{"replicaset", owner_name})
		} else if owner_kind == "ReplicationController" && workload == "<none>" {
			return series.Strings([]string{"replicationcontroller", owner_name})
		} else {
			return series.Strings([]string{workload_type, workload})
		}
	})

	df = df.Mutate(s.Col("X0")).Rename("k8s_object_type", "X0")
	df = df.Mutate(s.Col("X1")).Rename("k8s_object_name", "X1")
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
	return df
}
