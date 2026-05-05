package api

import "fmt"

const timeLayout = "2006-01-02"

type ParamError struct {
	AppErr  error
	UserErr bool
}

func (e *ParamError) Error() string { return e.AppErr.Error() }
func (e *ParamError) Unwrap() error { return e.AppErr }

// namespaceAPIErrf constructs a ParamError. UserErr per error is not evaluated currently;
// apiErrResponse in utils.go is the single gate controlling user-facing error visibility.
func namespaceAPIErrf(userErr bool, format string, args ...any) *ParamError { //nolint:unparam
	return &ParamError{AppErr: fmt.Errorf(format, args...), UserErr: userErr}
}

// Filter modes for param-based query filters (cluster, project, etc.).
const (
	FilterModeInclude = "include"
	FilterModeExact   = "exact"
	FilterModeExclude = "exclude"
)

const (
	SkipSanitizationForContainer = true
	SkipSanitizationForNamespace = true
)

const EnableUserAPIErr = false

// validWorkloadTypes is the fixed set of allowed workload_type values (mirrors the sorted_workloadtype DB enum).
var validWorkloadTypes = map[string]bool{
	"daemonset":             true,
	"deployment":            true,
	"deploymentconfig":      true,
	"replicaset":            true,
	"replicationcontroller": true,
	"statefulset":           true,
}

func validateWorkloadTypeValues(vals []string) error {
	for _, v := range vals {
		if !validWorkloadTypes[v] {
			return namespaceAPIErrf(EnableUserAPIErr, "invalid workload_type %q, must be one of: daemonset, deployment, deploymentconfig, replicaset, replicationcontroller, statefulset", v)
		}
	}
	return nil
}

// FilterModeClause maps mode to SQL clause suffix, wrap for include, and join for multi-value params.
var FilterModeClause = map[string]struct {
	Suffix string
	Wrap   bool
	Join   string
}{
	FilterModeInclude: {" ILIKE ?", true, " OR "},
	FilterModeExact:   {" = ?", false, " OR "},
	FilterModeExclude: {" != ?", false, " AND "},
}

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
