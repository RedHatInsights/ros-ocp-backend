package processor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConvertDateToISO8601(t *testing.T) {
	date := "2022-11-01 18:25:43 +0000 UTC"
	expected_result := "2022-11-01T18:25:43.000Z"
	result := convertDateToISO8601(date)

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
	result := convert2DarrayToMap(arr)
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
	result, _ := readCSVFromUrl(server.URL)
	expected_result := [][]string{{"container_name", "cpu_request_avg_container"}, {"ros", "23"}, {"test_container", "24"}}
	if diff := cmp.Diff(result, expected_result); diff != "" {
		t.Error(diff)
	}
}

func TestConvertStringToTime(t *testing.T) {
	input_data := "2022-11-01 01:00:00 +0000 UTC"
	result, _ := convertStringToTime(input_data)
	if result.String() != input_data {
		t.Errorf("Output %q not equal to expected %q", result.String(), input_data)
	}
}
