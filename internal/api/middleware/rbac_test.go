package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/spf13/viper"
)

func rbacDataFromJSON(t *testing.T, jsonStr string) []types.RbacData {
	t.Helper()
	var data []types.RbacData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("failed to unmarshal RBAC fixture: %v", err)
	}
	return data
}

// --- PERM tests: aggregate_permissions ---.

// UT-RBAC-PERM-001: Unrestricted cluster access grants wildcard.
func TestAggPermUnrestrictedCluster(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.cluster:read"}]`)
	perms := ExportAggregatePermissions(acls)
	if len(perms) != 1 {
		t.Fatalf("UT-RBAC-PERM-001: expected 1 key, got %d: %v", len(perms), perms)
	}
	vals := perms["openshift.cluster"]
	if len(vals) != 1 || vals[0] != "*" {
		t.Errorf("UT-RBAC-PERM-001: expected [\"*\"], got %v", vals)
	}
}

// UT-RBAC-PERM-002: Cluster access scoped to single UUID.
func TestAggPermClusterSingleUUID(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.cluster:read","resourceDefinitions":[{"attributeFilter":{"Key":"uuid","Value":"uuid-abc","Operation":"equal"}}]}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.cluster"]
	if len(vals) != 1 || vals[0] != "uuid-abc" {
		t.Errorf("UT-RBAC-PERM-002: expected [\"uuid-abc\"], got %v", vals)
	}
}

// UT-RBAC-PERM-003: Cluster access scoped to multiple UUIDs (array value).
func TestAggPermClusterMultipleUUIDs(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.cluster:read","resourceDefinitions":[{"attributeFilter":{"Key":"uuid","Value":["uuid-1","uuid-2"],"Operation":"in"}}]}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.cluster"]
	if len(vals) != 2 || vals[0] != "uuid-1" || vals[1] != "uuid-2" {
		t.Errorf("UT-RBAC-PERM-003: expected [\"uuid-1\", \"uuid-2\"], got %v", vals)
	}
}

// UT-RBAC-PERM-004: Project access scoped to single namespace.
func TestAggPermProjectSingleNamespace(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.project:read","resourceDefinitions":[{"attributeFilter":{"Key":"namespace","Value":"my-namespace","Operation":"equal"}}]}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.project"]
	if len(vals) != 1 || vals[0] != "my-namespace" {
		t.Errorf("UT-RBAC-PERM-004: expected [\"my-namespace\"], got %v", vals)
	}
}

// UT-RBAC-PERM-005: Unrestricted node access grants wildcard.
func TestAggPermUnrestrictedNode(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.node:read"}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.node"]
	if len(vals) != 1 || vals[0] != "*" {
		t.Errorf("UT-RBAC-PERM-005: expected [\"*\"], got %v", vals)
	}
}

// UT-RBAC-PERM-006: Wildcard resource type grants global access.
func TestAggPermWildcardType(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:*:read"}]`)
	perms := ExportAggregatePermissions(acls)
	vals, exists := perms["*"]
	if !exists {
		t.Fatal("UT-RBAC-PERM-006: expected key \"*\" in permissions")
	}
	if len(vals) != 0 {
		t.Errorf("UT-RBAC-PERM-006: expected empty slice for wildcard, got %v", vals)
	}
}

// UT-RBAC-PERM-007: Non-openshift types are ignored.
func TestAggPermNonOpenshiftIgnored(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:cost_model:write"}]`)
	perms := ExportAggregatePermissions(acls)
	if len(perms) != 0 {
		t.Errorf("UT-RBAC-PERM-007: expected empty map, got %v", perms)
	}
}

// UT-RBAC-PERM-008: No ACLs yields empty map.
func TestAggPermNoACLs(t *testing.T) {
	perms := ExportAggregatePermissions([]types.RbacData{})
	if len(perms) != 0 {
		t.Errorf("UT-RBAC-PERM-008: expected empty map, got %v", perms)
	}
}

// UT-RBAC-PERM-009: Multiple ACLs for same type accumulate scopes.
func TestAggPermAccumulateScopes(t *testing.T) {
	acls := rbacDataFromJSON(t, `[
		{"Permission":"cost-management:openshift.cluster:read","resourceDefinitions":[{"attributeFilter":{"Key":"uuid","Value":"uuid-1","Operation":"equal"}}]},
		{"Permission":"cost-management:openshift.cluster:read","resourceDefinitions":[{"attributeFilter":{"Key":"uuid","Value":"uuid-2","Operation":"equal"}}]}
	]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.cluster"]
	if len(vals) != 2 || vals[0] != "uuid-1" || vals[1] != "uuid-2" {
		t.Errorf("UT-RBAC-PERM-009: expected [\"uuid-1\", \"uuid-2\"], got %v", vals)
	}
}

// UT-RBAC-PERM-010: Mixed types from single RBAC response.
func TestAggPermMixedTypes(t *testing.T) {
	acls := rbacDataFromJSON(t, `[
		{"Permission":"cost-management:openshift.cluster:read"},
		{"Permission":"cost-management:openshift.project:read","resourceDefinitions":[{"attributeFilter":{"Key":"ns","Value":"ns-1","Operation":"equal"}}]},
		{"Permission":"cost-management:*:read"}
	]`)
	perms := ExportAggregatePermissions(acls)
	if len(perms) != 3 {
		t.Errorf("UT-RBAC-PERM-010: expected 3 keys, got %d: %v", len(perms), perms)
	}
	if vals := perms["openshift.cluster"]; len(vals) != 1 || vals[0] != "*" {
		t.Errorf("UT-RBAC-PERM-010: cluster expected [\"*\"], got %v", vals)
	}
	if vals := perms["openshift.project"]; len(vals) != 1 || vals[0] != "ns-1" {
		t.Errorf("UT-RBAC-PERM-010: project expected [\"ns-1\"], got %v", vals)
	}
	if _, exists := perms["*"]; !exists {
		t.Error("UT-RBAC-PERM-010: expected wildcard key")
	}
}

// UT-RBAC-PERM-011: Multiple resource definitions on single ACL.
func TestAggPermMultipleResourceDefs(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"cost-management:openshift.cluster:read","resourceDefinitions":[
		{"attributeFilter":{"Key":"uuid","Value":"uuid-a","Operation":"equal"}},
		{"attributeFilter":{"Key":"uuid","Value":"uuid-b","Operation":"equal"}}
	]}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.cluster"]
	if len(vals) != 2 || vals[0] != "uuid-a" || vals[1] != "uuid-b" {
		t.Errorf("UT-RBAC-PERM-011: expected [\"uuid-a\", \"uuid-b\"], got %v", vals)
	}
}

// UT-RBAC-PERM-012: Extra colons in permission string.
func TestAggPermExtraColons(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"app:openshift.cluster:read:extra"}]`)
	perms := ExportAggregatePermissions(acls)
	vals := perms["openshift.cluster"]
	if len(vals) != 1 || vals[0] != "*" {
		t.Errorf("UT-RBAC-PERM-012: expected [\"*\"], got %v", vals)
	}
}

// UT-RBAC-PERM-013: No-colon permission string is gracefully skipped.
func TestAggPermNoColonSkipped(t *testing.T) {
	acls := rbacDataFromJSON(t, `[{"Permission":"openshift"}]`)
	perms := ExportAggregatePermissions(acls)
	if len(perms) != 0 {
		t.Errorf("UT-RBAC-PERM-013: expected empty map for malformed permission, got %v", perms)
	}
}

// --- ACCESS tests: RBAC middleware via httptest ---.

func rbacHTTPTestServer(t *testing.T, statusCode int, responseBody interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if responseBody != nil {
			b, _ := json.Marshal(responseBody)
			_, _ = w.Write(b)
		}
	}))
}

func setupRBACConfig(t *testing.T, server *httptest.Server) {
	t.Helper()
	viper.Reset()
	config.ResetConfig()

	// Parse server URL to get host and port
	host := server.Listener.Addr().String()

	c := config.GetConfig()
	c.RBACHost = host
	c.RBACPort = ""
	c.RBACProtocol = "http"
	c.RBACEnabled = true
	cfg = c
}

func makeRbacIdentityHeader(orgID, username string) string { //nolint:unparam
	id := map[string]interface{}{
		"identity": map[string]interface{}{
			"org_id": orgID,
			"user":   map[string]interface{}{"username": username},
		},
	}
	b, _ := json.Marshal(id)
	return base64.StdEncoding.EncodeToString(b)
}

func invokeRbacMiddleware(t *testing.T, identityHeader string) (*httptest.ResponseRecorder, echo.Context) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Rh-Identity", identityHeader)
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}

	err := Rbac(handler)(c)
	if he, ok := err.(*echo.HTTPError); ok {
		rec.Code = he.Code
	}

	return rec, c
}

// UT-RBAC-ACCESS-001: User with RBAC permissions can proceed.
func TestRbacMiddlewareAccessAllowed(t *testing.T) {
	server := rbacHTTPTestServer(t, http.StatusOK, types.RbacResponse{
		Data: []types.RbacData{
			{Permission: "cost-management:openshift.cluster:read"},
		},
	})
	defer server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, c := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusOK {
		t.Errorf("UT-RBAC-ACCESS-001: expected HTTP 200, got %d", rec.Code)
	}
	perms, ok := c.Get("user.permissions").(map[string][]string)
	if !ok {
		t.Fatal("UT-RBAC-ACCESS-001: user.permissions not set or wrong type")
	}
	if vals := perms["openshift.cluster"]; len(vals) != 1 || vals[0] != "*" {
		t.Errorf("UT-RBAC-ACCESS-001: expected cluster [\"*\"], got %v", vals)
	}
}

// UT-RBAC-ACCESS-002: Empty RBAC data denies access.
func TestRbacMiddlewareEmptyDataDenied(t *testing.T) {
	server := rbacHTTPTestServer(t, http.StatusOK, types.RbacResponse{
		Data: []types.RbacData{},
	})
	defer server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, _ := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("UT-RBAC-ACCESS-002: expected HTTP 403, got %d", rec.Code)
	}
}

// UT-RBAC-ACCESS-003: Non-openshift permissions denied.
func TestRbacMiddlewareNonOpenshiftDenied(t *testing.T) {
	server := rbacHTTPTestServer(t, http.StatusOK, types.RbacResponse{
		Data: []types.RbacData{
			{Permission: "cost-management:cost_model:write"},
		},
	})
	defer server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, _ := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("UT-RBAC-ACCESS-003: expected HTTP 403, got %d", rec.Code)
	}
}

// UT-RBAC-ACCESS-004: Paginated RBAC response includes all pages.
func TestRbacMiddlewarePagination(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		var resp interface{}
		if requestCount == 1 {
			resp = map[string]interface{}{
				"links": map[string]interface{}{
					"next": "/api/rbac/v1/access/?page=2",
				},
				"data": []map[string]interface{}{
					{"permission": "cost-management:openshift.cluster:read"},
				},
			}
		} else {
			resp = map[string]interface{}{
				"links": map[string]interface{}{},
				"data": []map[string]interface{}{
					{"permission": "cost-management:openshift.project:read"},
				},
			}
		}
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, c := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusOK {
		t.Errorf("UT-RBAC-ACCESS-004: expected HTTP 200, got %d", rec.Code)
	}
	perms, ok := c.Get("user.permissions").(map[string][]string)
	if !ok {
		t.Fatal("UT-RBAC-ACCESS-004: user.permissions not set")
	}
	if _, exists := perms["openshift.cluster"]; !exists {
		t.Error("UT-RBAC-ACCESS-004: missing openshift.cluster permission from page 1")
	}
	if _, exists := perms["openshift.project"]; !exists {
		t.Error("UT-RBAC-ACCESS-004: missing openshift.project permission from page 2")
	}
}

// UT-RBAC-ACCESS-005: RBAC API server error denies access.
func TestRbacMiddlewareServerErrorDenied(t *testing.T) {
	server := rbacHTTPTestServer(t, http.StatusInternalServerError, nil)
	defer server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, _ := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("UT-RBAC-ACCESS-005: expected HTTP 403, got %d", rec.Code)
	}
}

// UT-RBAC-ACCESS-006: RBAC API network failure denies access gracefully.
func TestRbacMiddlewareNetworkFailureDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	setupRBACConfig(t, server)
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	rec, _ := invokeRbacMiddleware(t, makeRbacIdentityHeader("org-1", "user-1"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("UT-RBAC-ACCESS-006: expected HTTP 403 on network failure, got %d", rec.Code)
	}
}
