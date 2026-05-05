package api

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMapQueryParameters(t *testing.T) {
	// setup
	type tests struct {
		name     string
		qinputs  map[string]string
		qoutputs map[string]interface{}
		errmsg   string
	}

	now := time.Now().UTC().Truncate(time.Second)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	startTime := time.Date(2023, 3, 23, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 3, 24, 0, 0, 0, 0, time.UTC)
	inclusiveEndTime := endTime.Add(24 * time.Hour)

	all_tests := []tests{
		{
			name:    "When start date and end date are not provided",
			qinputs: map[string]string{"start_date": "", "end_date": ""},
			qoutputs: map[string]interface{}{
				"recommendation_sets.monitoring_end_time < ?":  now,
				"recommendation_sets.monitoring_end_time >= ?": firstOfMonth,
			},
			errmsg: `The startTime should be 1st of current month. The endTime should the current time.`,
		},
		{
			name:    "When start date and end date are provided",
			qinputs: map[string]string{"start_date": startTime.Format("2006-01-02"), "end_date": endTime.Format("2006-01-02")},
			qoutputs: map[string]interface{}{
				"recommendation_sets.monitoring_end_time < ?":  inclusiveEndTime,
				"recommendation_sets.monitoring_end_time >= ?": startTime,
			},
			errmsg: `The recommendation_sets.monitoring_end_time should be less than or equal to end date!
				The recommendation_sets.monitoring_end_time should be greater than or equal to start date!`,
		},
	}

	for _, tt := range all_tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh request and recorder for each parallel test
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			for k, v := range tt.qinputs {
				c.QueryParams().Add(k, v)
			}
			defer func() {
				// Cleanup query params regardless of test result
				for k := range c.QueryParams() {
					delete(c.QueryParams(), k)
				}
			}()
			result, _ := MapQueryParameters(c)
			if reflect.DeepEqual(result, tt.qoutputs) != true {
				t.Errorf("%s", tt.errmsg)
			}
		})
	}
}

func TestMapQueryParametersFilterClauses(t *testing.T) {
	containerCol := "recommendation_sets.container_name"
	workloadCol := "workloads.workload_name"
	workloadTypeCol := "workloads.workload_type"
	projectContainerCol := "workloads.namespace"

	tests := []struct {
		name        string
		queryParams map[string][]string
		wantErr     bool
		errContains string
		checkResult func(t *testing.T, result map[string]interface{})
	}{
		{
			name:        "container include (partial match)",
			queryParams: map[string][]string{"container": {"web-server"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"%web-server%"}, result[containerCol+" ILIKE ?"])
			},
		},
		{
			name:        "container exact match",
			queryParams: map[string][]string{"filter[exact:container]": {"web-server"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"web-server"}, result[containerCol+" = ?"])
			},
		},
		{
			name:        "container exclude",
			queryParams: map[string][]string{"exclude[container]": {"web-server"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"web-server"}, result[containerCol+" != ?"])
			},
		},
		{
			name: "container exclude and exact together",
			queryParams: map[string][]string{
				"filter[exact:container]": {"api-server"},
				"exclude[container]":      {"web-server"},
			},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"api-server"}, result[containerCol+" = ?"])
				assert.Equal(t, []string{"web-server"}, result[containerCol+" != ?"])
			},
		},
		{
			name:        "workload exact match",
			queryParams: map[string][]string{"filter[exact:workload]": {"my-deploy"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"my-deploy"}, result[workloadCol+" = ?"])
			},
		},
		{
			name:        "workload exclude",
			queryParams: map[string][]string{"exclude[workload]": {"my-deploy"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"my-deploy"}, result[workloadCol+" != ?"])
			},
		},
		{
			name:        "workload_type plain param uses exact match not partial",
			queryParams: map[string][]string{"workload_type": {"deployment"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"deployment"}, result[workloadTypeCol+" = ?"])
				assert.Nil(t, result[workloadTypeCol+" ILIKE ?"], "workload_type plain param must not use ILIKE")
			},
		},
		{
			name:        "workload_type exact match",
			queryParams: map[string][]string{"filter[exact:workload_type]": {"deployment"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"deployment"}, result[workloadTypeCol+" = ?"])
			},
		},
		{
			name:        "workload_type exclude",
			queryParams: map[string][]string{"exclude[workload_type]": {"daemonset"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"daemonset"}, result[workloadTypeCol+" != ?"])
			},
		},
		{
			name:        "workload_type invalid value returns error",
			queryParams: map[string][]string{"workload_type": {"not-a-real-type"}},
			wantErr:     true,
			errContains: "invalid workload_type",
		},
		{
			name:        "project exact match",
			queryParams: map[string][]string{"filter[exact:project]": {"default"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"default"}, result[projectContainerCol+" = ?"])
			},
		},
		{
			name:        "project exclude",
			queryParams: map[string][]string{"exclude[project]": {"kube-system"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, []string{"kube-system"}, result[projectContainerCol+" != ?"])
			},
		},
		{
			name:        "exclude and exact same container value - error",
			queryParams: map[string][]string{"exclude[container]": {"web"}, "filter[exact:container]": {"web"}},
			wantErr:     true,
			errContains: "exclude and exact cannot share values",
		},
		{
			name:        "container partial match multiple values",
			queryParams: map[string][]string{"container": {"web-server", "api-server"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				key := containerCol + " ILIKE ? OR " + containerCol + " ILIKE ?"
				assert.Equal(t, []string{"%web-server%", "%api-server%"}, result[key])
			},
		},
		{
			name:        "container exact match multiple values",
			queryParams: map[string][]string{"filter[exact:container]": {"web-server", "api-server"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				key := containerCol + " = ? OR " + containerCol + " = ?"
				assert.Equal(t, []string{"web-server", "api-server"}, result[key])
			},
		},
		{
			name:        "workload partial match multiple values",
			queryParams: map[string][]string{"workload": {"cart-svc", "pay-svc"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				key := workloadCol + " ILIKE ? OR " + workloadCol + " ILIKE ?"
				assert.Equal(t, []string{"%cart-svc%", "%pay-svc%"}, result[key])
			},
		},
		{
			name:        "project partial match multiple values",
			queryParams: map[string][]string{"project": {"ns-alpha", "ns-beta"}},
			checkResult: func(t *testing.T, result map[string]interface{}) {
				key := projectContainerCol + " ILIKE ? OR " + projectContainerCol + " ILIKE ?"
				assert.Equal(t, []string{"%ns-alpha%", "%ns-beta%"}, result[key])
			},
		},
		{
			name:        "container exceeds max length",
			queryParams: map[string][]string{"container": {strings.Repeat("a", model.NamespaceMaxLen+1)}},
			wantErr:     false,
		},
		{
			name:        "project exceeds max length",
			queryParams: map[string][]string{"project": {strings.Repeat("a", model.NamespaceMaxLen+1)}},
			wantErr:     false,
		},
		{
			name: "multiple filters exceed max length - joined errors",
			queryParams: map[string][]string{
				"container": {strings.Repeat("a", model.NamespaceMaxLen+1)},
				"project":   {strings.Repeat("b", model.NamespaceMaxLen+1)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			for k, vals := range tt.queryParams {
				for _, v := range vals {
					c.QueryParams().Add(k, v)
				}
			}
			result, err := MapQueryParameters(c)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

var FlattenedCSVHeaderFixture = []string{
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

func TestFlattenedCSVHeader(t *testing.T) {
	header := FlattenedCSVHeader
	assert.Len(t, header, len(FlattenedCSVHeaderFixture), "header length mismatch")
	assert.Equal(t, FlattenedCSVHeaderFixture, header, "header content or order is incorrect")
}

func buildClauseForParam(param string, includeVals, exactVals, excludeVals []string, column string) (map[string]any, error) {
	maxLen, allowDot := model.NamespaceMaxLen, false
	if param == "cluster" {
		maxLen, allowDot = model.ClusterMaxLen, true
	}
	return buildSQLClauseWithFilterType(param, includeVals, exactVals, excludeVals, column, maxLen, allowDot, false)
}

func TestBuildSQLClauseWithFilterType(t *testing.T) {
	projectCol := "namespace_recommendation_sets.namespace_name"

	tests := []struct {
		name        string
		param       string
		includeVals []string
		exactVals   []string
		excludeVals []string
		column      string
		wantErr     bool
		errContains string
		checkClause func(t *testing.T, clause map[string]any)
	}{
		{
			name:        "include only, project",
			param:       "project",
			includeVals: []string{"foo"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				key := projectCol + " ILIKE ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"%foo%"}, clause[key])
			},
		},
		{
			name:      "exact only, project",
			param:     "project",
			exactVals: []string{"foo"},
			column:    projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				key := projectCol + " = ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"foo"}, clause[key])
			},
		},
		{
			name:        "exclude only, project",
			param:       "project",
			excludeVals: []string{"foo"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				key := projectCol + " != ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"foo"}, clause[key])
			},
		},
		{
			name:      "cluster with UUID, exact",
			param:     "cluster",
			exactVals: []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"},
			column:    "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				assert.Contains(t, clause, "clusters.cluster_uuid = ?")
				assert.Equal(t, []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"}, clause["clusters.cluster_uuid = ?"])
			},
		},
		{
			name:        "cluster with alias, include",
			param:       "cluster",
			includeVals: []string{"foo-cluster"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				assert.Contains(t, clause, "clusters.cluster_alias ILIKE ?")
				assert.Equal(t, []string{"%foo-cluster%"}, clause["clusters.cluster_alias ILIKE ?"])
			},
		},
		{
			name:        "cluster with alias, exclude",
			param:       "cluster",
			excludeVals: []string{"foo-cluster"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				assert.Contains(t, clause, "clusters.cluster_alias != ?")
				assert.Equal(t, []string{"foo-cluster"}, clause["clusters.cluster_alias != ?"])
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			clause, err := buildClauseForParam(tt.param, tt.includeVals, tt.exactVals, tt.excludeVals, tt.column)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			if tt.checkClause != nil {
				tt.checkClause(t, clause)
			}
		})
	}
}

func TestBuildSQLClauseWithFilterTypeConflict(t *testing.T) {
	projectCol := "namespace_recommendation_sets.namespace_name"

	tests := []struct {
		name        string
		param       string
		includeVals []string
		exactVals   []string
		excludeVals []string
		column      string
		errContains string
	}{
		{
			name:        "exclude and include same value - error",
			param:       "project",
			includeVals: []string{"foo"},
			excludeVals: []string{"foo"},
			column:      projectCol,
			errContains: "exclude and include cannot share values",
		},
		{
			name:        "exclude and exact same value - error",
			param:       "cluster",
			exactVals:   []string{"prod-cluster"},
			excludeVals: []string{"prod-cluster"},
			column:      "",
			errContains: "exclude and exact cannot share values",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildClauseForParam(tt.param, tt.includeVals, tt.exactVals, tt.excludeVals, tt.column)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestBuildSQLClauseWithFilterTypeMultipleParams(t *testing.T) {
	projectCol := "namespace_recommendation_sets.namespace_name"

	tests := []struct {
		name        string
		param       string
		includeVals []string
		exactVals   []string
		excludeVals []string
		column      string
		checkClause func(t *testing.T, clause map[string]any)
	}{
		{
			name:        "project exact and include same value - exact priority",
			param:       "project",
			includeVals: []string{"foo"},
			exactVals:   []string{"foo"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				assert.Equal(t, []string{"foo"}, clause[projectCol+" = ?"])
			},
		},
		{
			name:        "project exact and include different values - both",
			param:       "project",
			includeVals: []string{"foo"},
			exactVals:   []string{"bar"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 2)
				assert.Equal(t, []string{"bar"}, clause[projectCol+" = ?"])
				assert.Equal(t, []string{"%foo%"}, clause[projectCol+" ILIKE ?"])
			},
		},
		{
			name:        "project include multiple values",
			param:       "project",
			includeVals: []string{"foo", "bar"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				key := projectCol + " ILIKE ? OR " + projectCol + " ILIKE ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"%foo%", "%bar%"}, clause[key])
			},
		},
		{
			name:        "project exclude and include different values",
			param:       "project",
			includeVals: []string{"foo"},
			excludeVals: []string{"bar"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 2)
				assert.Equal(t, []string{"bar"}, clause[projectCol+" != ?"])
				assert.Equal(t, []string{"%foo%"}, clause[projectCol+" ILIKE ?"])
			},
		},
		{
			name:        "cluster include and exact different values - both",
			param:       "cluster",
			includeVals: []string{"prod"},
			exactVals:   []string{"dev"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 2)
				assert.Equal(t, []string{"%prod%"}, clause["clusters.cluster_alias ILIKE ?"])
				assert.Equal(t, []string{"dev"}, clause["clusters.cluster_alias = ?"])
			},
		},
		{
			name:        "cluster include, exact, exclude - all different values",
			param:       "cluster",
			includeVals: []string{"prod"},
			exactVals:   []string{"dev"},
			excludeVals: []string{"staging"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 3)
				assert.Equal(t, []string{"staging"}, clause["clusters.cluster_alias != ?"])
				assert.Equal(t, []string{"dev"}, clause["clusters.cluster_alias = ?"])
				assert.Equal(t, []string{"%prod%"}, clause["clusters.cluster_alias ILIKE ?"])
			},
		},
		{
			name:        "project include, exact, exclude - all different values",
			param:       "project",
			includeVals: []string{"foo"},
			exactVals:   []string{"bar"},
			excludeVals: []string{"baz"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 3)
				assert.Equal(t, []string{"baz"}, clause[projectCol+" != ?"])
				assert.Equal(t, []string{"bar"}, clause[projectCol+" = ?"])
				assert.Equal(t, []string{"%foo%"}, clause[projectCol+" ILIKE ?"])
			},
		},
		{
			name:        "cluster UUID exact takes precedence over include when same value",
			param:       "cluster",
			includeVals: []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"},
			exactVals:   []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				assert.Len(t, clause, 1)
				assert.Contains(t, clause, "clusters.cluster_uuid = ?")
				assert.Equal(t, []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"}, clause["clusters.cluster_uuid = ?"])
			},
		},
		{
			name:        "cluster include multiple values",
			param:       "cluster",
			includeVals: []string{"prod", "dev"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				key := "clusters.cluster_alias ILIKE ? OR clusters.cluster_alias ILIKE ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"%prod%", "%dev%"}, clause[key])
			},
		},
		{
			name:        "cluster include multiple UUIDs",
			param:       "cluster",
			includeVals: []string{"1b36b20f-7fa0-4454-a6d2-008294e06378", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				key := "clusters.cluster_uuid = ? OR clusters.cluster_uuid = ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"1b36b20f-7fa0-4454-a6d2-008294e06378", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}, clause[key])
			},
		},
		{
			name:      "cluster exact multiple values",
			param:     "cluster",
			exactVals: []string{"prod", "dev"},
			column:    "",
			checkClause: func(t *testing.T, clause map[string]any) {
				key := "clusters.cluster_alias = ? OR clusters.cluster_alias = ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"prod", "dev"}, clause[key])
			},
		},
		{
			name:      "project exact multiple values",
			param:     "project",
			exactVals: []string{"foo", "bar"},
			column:    projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				key := projectCol + " = ? OR " + projectCol + " = ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"foo", "bar"}, clause[key])
			},
		},
		{
			name:        "project exclude multiple values",
			param:       "project",
			excludeVals: []string{"foo", "bar"},
			column:      projectCol,
			checkClause: func(t *testing.T, clause map[string]any) {
				key := projectCol + " != ? AND " + projectCol + " != ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"foo", "bar"}, clause[key])
			},
		},
		{
			name:        "cluster exclude multiple values",
			param:       "cluster",
			excludeVals: []string{"prod", "dev"},
			column:      "",
			checkClause: func(t *testing.T, clause map[string]any) {
				key := "clusters.cluster_alias != ? AND clusters.cluster_alias != ?"
				assert.Contains(t, clause, key)
				assert.Equal(t, []string{"prod", "dev"}, clause[key])
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			clause, err := buildClauseForParam(tt.param, tt.includeVals, tt.exactVals, tt.excludeVals, tt.column)
			assert.NoError(t, err)
			if tt.checkClause != nil {
				tt.checkClause(t, clause)
			}
		})
	}

	t.Run("cluster exact UUID and project exclude - both in single request", func(t *testing.T) {
		clusterClause, err := buildClauseForParam("cluster", nil, []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"}, nil, "")
		assert.NoError(t, err)
		projectClause, err := buildClauseForParam("project", nil, nil, []string{"baz"}, projectCol)
		assert.NoError(t, err)

		queryParams := make(map[string]any)
		maps.Copy(queryParams, clusterClause)
		maps.Copy(queryParams, projectClause)

		assert.Contains(t, queryParams, "clusters.cluster_uuid = ?")
		assert.Contains(t, queryParams, projectCol+" != ?")
		assert.Equal(t, []string{"1b36b20f-7fa0-4454-a6d2-008294e06378"}, queryParams["clusters.cluster_uuid = ?"])
		assert.Equal(t, []string{"baz"}, queryParams[projectCol+" != ?"])
	})
}
