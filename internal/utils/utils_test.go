package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestHTTPClientHasTimeout(t *testing.T) {
	if HTTPClient.Timeout == 0 {
		t.Fatal("HTTPClient.Timeout must be non-zero to prevent indefinite hangs (FLPATH-3407)")
	}
	if HTTPClient.Timeout > 60*time.Second {
		t.Errorf("HTTPClient.Timeout=%v is too high; expected <= 60s", HTTPClient.Timeout)
	}
}

func TestHTTPClientTimesOutOnSlowServer(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	// Use a very short timeout so the test runs fast
	HTTPClient = &http.Client{Timeout: 100 * time.Millisecond}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := HTTPClient.Get(server.URL) //nolint:bodyclose // response is nil on expected timeout
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected timeout error from slow server, got nil")
	}
}

func TestHTTPClientSucceedsOnFastServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	resp, err := HTTPClient.Get(server.URL)
	if err != nil {
		t.Fatalf("expected success from fast server, got error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestConvertDateToISO8601(t *testing.T) {
	date := "2022-11-01 18:25:43 +0000 UTC"
	expected_result := "2022-11-01T18:25:43.000Z"
	result := ConvertDateToISO8601(date)

	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
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
