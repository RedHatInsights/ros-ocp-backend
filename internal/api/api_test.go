package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Truncate(time.Second)

	startTime := time.Date(2023, 3, 23, 0, 0, 0, 0, time.UTC).Truncate(time.Second)
	endTime := time.Date(2023, 3, 24, 0, 0, 0, 0, time.UTC).Truncate(time.Second)

	all_tests := []tests{
		{
			name:    "When start date and end date are not provided",
			qinputs: map[string]string{"start_date": "", "end_date": ""},
			qoutputs: map[string]interface{}{
				"recommendation_sets.monitoring_end_time <= ?": now,
				"recommendation_sets.monitoring_end_time >= ?": firstOfMonth,
			},
			errmsg: `The startTime should be 1st of current month. The endTime should the current time.`,
		},
		{
			name:    "When start date and end date are provided",
			qinputs: map[string]string{"start_date": startTime.Format("2006-01-02"), "end_date": endTime.Format("2006-01-02")},
			qoutputs: map[string]interface{}{
				"recommendation_sets.monitoring_end_time <= ?": endTime,
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
