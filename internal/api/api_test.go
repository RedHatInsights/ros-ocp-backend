package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"github.com/labstack/echo/v4"
)

func TestMapQueryParameters(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	type tests struct {
		name     string
		qinputs  map[string]string
		qoutputs map[string][]string
		errmsg   string
	}

	var all_tests = []tests{
		{
			name:    "When start date and end date are provided",
			qinputs: map[string]string{"start_date": "2023-03-23", "end_date": "2023-03-24"},
			qoutputs: map[string][]string{
				"DATE(recommendation_sets.monitoring_end_time) <= ?": {"2023-03-24"},
				"DATE(recommendation_sets.monitoring_end_time) >= ?": {"2023-03-23"},
			},
			errmsg: `The recommendation_sets.monitoring_end_time should be less than or equal to end date!
				The recommendation_sets.monitoring_end_time should be greater than or equal to start date!`,
		},
	}

	for _, tt := range all_tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.qinputs {
				c.QueryParams().Add(k, v)
			}
			result, _ := MapQueryParameters(c)
			if reflect.DeepEqual(result, tt.qoutputs) != true {
				t.Errorf("%s", tt.errmsg)
			}
			for k := range c.QueryParams() {
				delete(c.QueryParams(), k)
			}
		})
	}
}
