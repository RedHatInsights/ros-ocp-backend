package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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
