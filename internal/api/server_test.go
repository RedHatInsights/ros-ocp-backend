package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTestEchoWithRecommendationRoutes() *echo.Echo {
	e := echo.New()
	v1 := e.Group("/api/cost-management/v1")
	registerRecommendationRoutes(v1)
	return e
}

func findRoute(t *testing.T, e *echo.Echo, path string) echo.Context {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	e.Router().Find(http.MethodGet, path, c)
	if c.Handler() == nil {
		t.Fatalf("no handler registered for %s %s", http.MethodGet, path)
	}
	return c
}

func TestContainerRecommendationRoutes_ResolveToSharedHandlers(t *testing.T) {
	e := newTestEchoWithRecommendationRoutes()
	recommendationID := "550e8400-e29b-41d4-a716-446655440000"

	tests := []struct {
		name         string
		path         string
		wantRoute    string
		wantParamKey string
		wantParamVal string
	}{
		{
			name:      "legacy container list",
			path:      "/api/cost-management/v1/recommendations/openshift",
			wantRoute: "/api/cost-management/v1/recommendations/openshift",
		},
		{
			name:      "explicit container list",
			path:      "/api/cost-management/v1/recommendations/openshift/container",
			wantRoute: "/api/cost-management/v1/recommendations/openshift/container",
		},
		{
			name:         "legacy container detail",
			path:         "/api/cost-management/v1/recommendations/openshift/" + recommendationID,
			wantRoute:    "/api/cost-management/v1/recommendations/openshift/:recommendation-id",
			wantParamKey: "recommendation-id",
			wantParamVal: recommendationID,
		},
		{
			name:         "explicit container detail",
			path:         "/api/cost-management/v1/recommendations/openshift/container/" + recommendationID,
			wantRoute:    "/api/cost-management/v1/recommendations/openshift/container/:recommendation-id",
			wantParamKey: "recommendation-id",
			wantParamVal: recommendationID,
		},
		{
			name:      "namespace list unchanged",
			path:      "/api/cost-management/v1/recommendations/openshift/namespace",
			wantRoute: "/api/cost-management/v1/recommendations/openshift/namespace",
		},
		{
			name:         "namespace detail unchanged",
			path:         "/api/cost-management/v1/recommendations/openshift/namespace/" + recommendationID,
			wantRoute:    "/api/cost-management/v1/recommendations/openshift/namespace/:recommendation-id",
			wantParamKey: "recommendation-id",
			wantParamVal: recommendationID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := findRoute(t, e, tt.path)
			if c.Path() != tt.wantRoute {
				t.Errorf("path %q matched route %q, want %q", tt.path, c.Path(), tt.wantRoute)
			}
			if tt.wantParamKey != "" && c.Param(tt.wantParamKey) != tt.wantParamVal {
				t.Errorf("param %q = %q, want %q", tt.wantParamKey, c.Param(tt.wantParamKey), tt.wantParamVal)
			}
		})
	}
}

func TestContainerRecommendationRoutes_StaticContainerSegmentNotLegacyDetail(t *testing.T) {
	e := newTestEchoWithRecommendationRoutes()

	c := findRoute(t, e, "/api/cost-management/v1/recommendations/openshift/container")
	wantRoute := "/api/cost-management/v1/recommendations/openshift/container"
	if c.Path() != wantRoute {
		t.Errorf("matched route %q, want %q", c.Path(), wantRoute)
	}
}
