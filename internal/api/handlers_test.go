package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupBrokenDB sets database.DB to an in-memory SQLite instance with no tables.
// Any query against recommendation_sets / namespace_recommendation_sets will fail.
func setupBrokenDB(t *testing.T) func() {
	t.Helper()
	origDB := database.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	database.DB = db
	return func() { database.DB = origDB }
}

func newHandlerContext(t *testing.T, method, path string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("Identity", identity.XRHID{
		Identity: identity.Identity{OrgID: "test-org"},
	})
	c.Set("user.permissions", map[string][]string{"*": {}})
	return c, rec
}

func TestGetRecommendationSetList_DBError_Returns503(t *testing.T) {
	restore := setupBrokenDB(t)
	defer restore()

	c, rec := newHandlerContext(t, http.MethodGet, "/api/v1/recommendations")

	err := GetRecommendationSetList(c)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	var body map[string]string
	if jsonErr := json.Unmarshal(rec.Body.Bytes(), &body); jsonErr != nil {
		t.Fatalf("failed to parse response body: %v", jsonErr)
	}
	if body["status"] != "error" {
		t.Errorf("expected status=error, got %q", body["status"])
	}
}

func TestGetNamespaceRecommendationSetList_DBError_Returns503(t *testing.T) {
	restore := setupBrokenDB(t)
	defer restore()

	c, rec := newHandlerContext(t, http.MethodGet, "/api/v1/openshift/namespace/recommendations")

	err := GetNamespaceRecommendationSetList(c)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable && EnableUserAPIErr {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	var body map[string]string
	if jsonErr := json.Unmarshal(rec.Body.Bytes(), &body); jsonErr != nil {
		t.Fatalf("failed to parse response body: %v", jsonErr)
	}
	if body["status"] != "error" && EnableUserAPIErr {
		t.Errorf("expected status=error, got %q", body["status"])
	}
}
