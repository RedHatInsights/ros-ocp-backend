package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestHTTPClientTimeoutMatchesConfig(t *testing.T) {
	secs := cfg.GlobalHTTPClientTimeoutSecs
	if secs < minHTTPTimeoutSecs {
		secs = minHTTPTimeoutSecs
	}
	expected := time.Duration(secs) * time.Second
	if HTTPClient.Timeout != expected {
		t.Errorf("HTTPClient.Timeout=%v; want %v (from GLOBAL_HTTP_CLIENT_TIMEOUT_SECS with floor %ds)",
			HTTPClient.Timeout, expected, minHTTPTimeoutSecs)
	}
}

func TestNewHTTPClientClampsZeroToFloor(t *testing.T) {
	client := newHTTPClient(0)
	floor := time.Duration(minHTTPTimeoutSecs) * time.Second
	if client.Timeout != floor {
		t.Errorf("newHTTPClient(0).Timeout=%v; want floor %v", client.Timeout, floor)
	}
}

func TestNewHTTPClientClampsNegativeToFloor(t *testing.T) {
	client := newHTTPClient(-5)
	floor := time.Duration(minHTTPTimeoutSecs) * time.Second
	if client.Timeout != floor {
		t.Errorf("newHTTPClient(-5).Timeout=%v; want floor %v", client.Timeout, floor)
	}
}

func TestNewHTTPClientRespectsValidValue(t *testing.T) {
	client := newHTTPClient(45)
	expected := 45 * time.Second
	if client.Timeout != expected {
		t.Errorf("newHTTPClient(45).Timeout=%v; want %v", client.Timeout, expected)
	}
}

func TestNewHTTPClientAtFloorBoundary(t *testing.T) {
	client := newHTTPClient(minHTTPTimeoutSecs)
	expected := time.Duration(minHTTPTimeoutSecs) * time.Second
	if client.Timeout != expected {
		t.Errorf("newHTTPClient(%d).Timeout=%v; want %v", minHTTPTimeoutSecs, client.Timeout, expected)
	}
}

func TestConvertDateToISO8601(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid UTC date",
			input:   "2022-11-01 18:25:43 +0000 UTC",
			want:    "2022-11-01T18:25:43.000Z",
			wantErr: false,
		},
		{
			name:    "valid non-UTC date",
			input:   "2023-06-15 09:30:00 +0530 IST",
			want:    "2023-06-15T09:30:00.000Z",
			wantErr: false,
		},
		{
			name:    "empty string returns error",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "garbage input returns error",
			input:   "not-a-date",
			want:    "",
			wantErr: true,
		},
		{
			name:    "wrong format returns error",
			input:   "2022-11-01",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertDateToISO8601(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertDateToISO8601(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertDateToISO8601(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvert2DarrayToMap(t *testing.T) {
	arr := [][]string{{"key1", "key2", "key3"}, {"value1", "value2", "value3"}}
	expected_result := []map[string]interface{}{{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}}
	result := Convert2DarrayToMap(arr)
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestUnique(t *testing.T) {
	arr := []int{1, 2, 3, 3, 4, 4, 5, 6, 6}
	expected_result := []int{1, 2, 3, 4, 5, 6}
	result := unique(arr)

	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestReadCSVFromUrl(t *testing.T) {
	testdata := "container_name,cpu_request_avg_container\nros,23\ntest_container,24"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testdata)
	}))
	defer server.Close()
	result, _ := ReadCSVFromUrl(server.URL)
	expected_result := [][]string{{"container_name", "cpu_request_avg_container"}, {"ros", "23"}, {"test_container", "24"}}
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestConvertStringToTime(t *testing.T) {
	input_data := "2022-11-01 01:00:00 +0000 UTC"
	result, _ := ConvertStringToTime(input_data)
	if result.String() != input_data {
		t.Errorf("Output %q not equal to expected %q", result.String(), input_data)
	}
}

func TestNeedRecommOnFirstOfMonth(t *testing.T) {
	layout := "2006-01-02 15:04:05"

	// Check condition if month change
	dbDate, _ := time.Parse(layout, "2023-11-30 11:45:26")
	maxEndTime, _ := time.Parse(layout, "2023-12-01 11:45:26")
	if !NeedRecommOnFirstOfMonth(dbDate, maxEndTime) {
		t.Errorf("NeedRecommOnFirstOfMonth fails for month change. dbDate=%s maxEndTime=%s", dbDate, maxEndTime)
	}

	// Check condition if year change
	dbDate, _ = time.Parse(layout, "2023-12-31 11:45:26")
	maxEndTime, _ = time.Parse(layout, "2024-01-01 11:45:26")
	if !NeedRecommOnFirstOfMonth(dbDate, maxEndTime) {
		t.Errorf("NeedRecommOnFirstOfMonth fails for year change. dbDate=%s maxEndTime=%s", dbDate, maxEndTime)
	}

	// Check condition if dbDate and maxEndTime both dates are first of month
	dbDate, _ = time.Parse(layout, "2023-12-01 10:45:26")
	maxEndTime, _ = time.Parse(layout, "2023-12-01 11:45:26")
	if NeedRecommOnFirstOfMonth(dbDate, maxEndTime) {
		t.Errorf("NeedRecommOnFirstOfMonth fails when both dbDate and maxEndTime date is first of month. dbDate=%s maxEndTime=%s", dbDate, maxEndTime)
	}

	// Check if it's not 1st of the month
	dbDate, _ = time.Parse(layout, "2023-11-29 10:45:26")
	maxEndTime, _ = time.Parse(layout, "2023-11-30 11:45:26")
	if NeedRecommOnFirstOfMonth(dbDate, maxEndTime) {
		t.Errorf("NeedRecommOnFirstOfMonth fails for condition maxEndTime not 1st of month. dbDate=%s maxEndTime=%s", dbDate, maxEndTime)
	}

}
