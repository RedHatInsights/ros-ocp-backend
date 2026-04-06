package sources

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/spf13/viper"
)

func resetSourcesTestEnv(t *testing.T) {
	t.Helper()
	_ = os.Unsetenv("COST_APPLICATION_TYPE_ID")
	viper.Reset()
	config.ResetConfig()
	cfg = nil
}

// UT-SRC-APPID-001: COST_APPLICATION_TYPE_ID=5 -> returns 5.
func TestGetCostAppIDFromEnvValid(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	_ = os.Setenv("COST_APPLICATION_TYPE_ID", "5")
	defer func() { _ = os.Unsetenv("COST_APPLICATION_TYPE_ID") }()

	id, err := GetCostApplicationID()
	if err != nil {
		t.Fatalf("UT-SRC-APPID-001: unexpected error: %v", err)
	}
	if id != 5 {
		t.Errorf("UT-SRC-APPID-001: expected 5, got %d", id)
	}
}

// UT-SRC-APPID-002: COST_APPLICATION_TYPE_ID=0 -> returns 0.
func TestGetCostAppIDFromEnvZero(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	_ = os.Setenv("COST_APPLICATION_TYPE_ID", "0")
	defer func() { _ = os.Unsetenv("COST_APPLICATION_TYPE_ID") }()

	id, err := GetCostApplicationID()
	if err != nil {
		t.Fatalf("UT-SRC-APPID-002: unexpected error: %v", err)
	}
	if id != 0 {
		t.Errorf("UT-SRC-APPID-002: expected 0, got %d", id)
	}
}

// UT-SRC-APPID-003: COST_APPLICATION_TYPE_ID=99999 -> returns 99999.
func TestGetCostAppIDFromEnvLarge(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	_ = os.Setenv("COST_APPLICATION_TYPE_ID", "99999")
	defer func() { _ = os.Unsetenv("COST_APPLICATION_TYPE_ID") }()

	id, err := GetCostApplicationID()
	if err != nil {
		t.Fatalf("UT-SRC-APPID-003: unexpected error: %v", err)
	}
	if id != 99999 {
		t.Errorf("UT-SRC-APPID-003: expected 99999, got %d", id)
	}
}

// UT-SRC-APPID-004: COST_APPLICATION_TYPE_ID=abc -> error containing "invalid".
func TestGetCostAppIDFromEnvInvalid(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	_ = os.Setenv("COST_APPLICATION_TYPE_ID", "abc")
	defer func() { _ = os.Unsetenv("COST_APPLICATION_TYPE_ID") }()

	id, err := GetCostApplicationID()
	if err == nil {
		t.Fatal("UT-SRC-APPID-004: expected error for non-numeric value, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("UT-SRC-APPID-004: error %q should contain 'invalid'", err.Error())
	}
	if id != 0 {
		t.Errorf("UT-SRC-APPID-004: expected 0 on error, got %d", id)
	}
}

func setupSourcesHTTPTest(t *testing.T, statusCode int, body interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		if body != nil {
			b, _ := json.Marshal(body)
			_, _ = w.Write(b)
		}
	}))
	// Override the package-level cfg to point to the test server
	_ = os.Setenv("CLOWDER_ENABLED", "false")
	config.ResetConfig()
	c := config.GetConfig()
	c.SourceApiBaseUrl = server.URL
	c.SourceApiPrefix = ""
	cfg = c
	return server
}

// UT-SRC-APPID-005: COST_APPLICATION_TYPE_ID="" (empty) -> falls through to HTTP, returns 7.
func TestGetCostAppIDFallsToHTTPOnEmptyEnv(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	_ = os.Setenv("COST_APPLICATION_TYPE_ID", "")

	server := setupSourcesHTTPTest(t, http.StatusOK, map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"id": "7"},
		},
	})
	defer server.Close()

	id, err := GetCostApplicationID()
	if err != nil {
		t.Fatalf("UT-SRC-APPID-005: unexpected error: %v", err)
	}
	if id != 7 {
		t.Errorf("UT-SRC-APPID-005: expected 7, got %d", id)
	}
}

// UT-SRC-APPID-006: env var unset -> falls through to HTTP, returns 3.
func TestGetCostAppIDFallsToHTTPOnUnset(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	server := setupSourcesHTTPTest(t, http.StatusOK, map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"id": "3"},
		},
	})
	defer server.Close()

	id, err := GetCostApplicationID()
	if err != nil {
		t.Fatalf("UT-SRC-APPID-006: unexpected error: %v", err)
	}
	if id != 3 {
		t.Errorf("UT-SRC-APPID-006: expected 3, got %d", id)
	}
}

// UT-SRC-APPID-007: env var unset, httptest returns 404 -> error.
func TestGetCostAppIDHTTP404(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	server := setupSourcesHTTPTest(t, http.StatusNotFound, nil)
	defer server.Close()

	_, err := GetCostApplicationID()
	if err == nil {
		t.Fatal("UT-SRC-APPID-007: expected error for HTTP 404, got nil")
	}
}

// UT-SRC-APPID-008: env var unset, httptest returns garbage -> error.
func TestGetCostAppIDHTTPGarbage(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	config.ResetConfig()
	c := config.GetConfig()
	c.SourceApiBaseUrl = server.URL
	c.SourceApiPrefix = ""
	cfg = c

	_, err := GetCostApplicationID()
	if err == nil {
		t.Fatal("UT-SRC-APPID-008: expected error for garbage response, got nil")
	}
}

// UT-SRC-APPID-009: env var unset, httptest returns {"data": []} -> error.
func TestGetCostAppIDEmptyDataReturnsError(t *testing.T) {
	resetSourcesTestEnv(t)
	t.Cleanup(func() { resetSourcesTestEnv(t) })

	server := setupSourcesHTTPTest(t, http.StatusOK, map[string]interface{}{
		"data": []interface{}{},
	})
	defer server.Close()

	id, err := GetCostApplicationID()
	if err == nil {
		t.Fatal("UT-SRC-APPID-009: expected error for empty data array, got nil")
	}
	if id != 0 {
		t.Errorf("UT-SRC-APPID-009: expected 0 on error, got %d", id)
	}
}
