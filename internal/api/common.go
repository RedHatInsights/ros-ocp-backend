package api

const timeLayout = "2006-01-02"

type Collection struct {
	Data  []interface{} `json:"data"`
	Meta  Metadata      `json:"meta"`
	Links Links         `json:"links"`
}

type Metadata struct {
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Links struct {
	First    string `json:"first"`
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next,omitempty"`
	Last     string `json:"last"`
}

var NotificationsToShow = map[string]string{
	"323004": "NOTICE",
	"323005": "NOTICE",
	"324003": "NOTICE",
	"324004": "NOTICE",
}

var MemoryUnitk8s = map[string]string{
	"bytes": "bytes",
	"MiB":   "Mi",
	"GiB":   "Gi",
}

var CPUUnitk8s = map[string]string{
	"millicores": "m",
	"cores":      "",
}

var FlattenedCSVHeader = []string{
	"id",
	"cluster_uuid",
	"cluster_alias",
	"container",
	"project",
	"workload",
	"workload_type",
	"last_reported",
	"source_id",
	"current_cpu_limit_amount",
	"current_cpu_limit_format",
	"current_memory_limit_amount",
	"current_memory_limit_format",
	"current_cpu_request_amount",
	"current_cpu_request_format",
	"current_memory_request_amount",
	"current_memory_request_format",
	"monitoring_end_time",
	"recommendation_term",
	"duration_in_hours",
	"monitoring_start_time",
	"recommendation_type",
	"config_cpu_limit_amount",
	"config_cpu_limit_format",
	"config_memory_limit_amount",
	"config_memory_limit_format",
	"config_cpu_request_amount",
	"config_cpu_request_format",
	"config_memory_request_amount",
	"config_memory_request_format",
	"variation_cpu_limit_amount",
	"variation_cpu_limit_format",
	"variation_memory_limit_amount",
	"variation_memory_limit_format",
	"variation_cpu_request_amount",
	"variation_cpu_request_format",
	"variation_memory_request_amount",
	"variation_memory_request_format",
}
