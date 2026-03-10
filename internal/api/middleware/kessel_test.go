package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// inputSensitiveMock implements kessel.PermissionChecker for unit tests.
type inputSensitiveMock struct {
	allowedTuples map[string]bool
	errorTuples   map[string]error
	authorizedIDs map[string][]string
	listErrors    map[string]error
}

func (m *inputSensitiveMock) CheckPermission(_ context.Context, orgID, permission, username string) (bool, error) {
	key := orgID + "|" + permission + "|" + username
	if err, ok := m.errorTuples[key]; ok {
		return false, err
	}
	return m.allowedTuples[key], nil
}

func (m *inputSensitiveMock) ListAuthorizedResources(_ context.Context, orgID, resourceType, permission, username string) ([]string, error) {
	key := orgID + "|" + resourceType + "|" + permission + "|" + username
	if m.listErrors != nil {
		if err, ok := m.listErrors[key]; ok {
			return nil, err
		}
	}
	if m.authorizedIDs != nil {
		if ids, ok := m.authorizedIDs[key]; ok {
			return ids, nil
		}
	}
	return []string{}, nil
}

func encodeIdentity(orgID, username string) string {
	id := identity.XRHID{
		Identity: identity.Identity{
			OrgID: orgID,
			User:  identity.User{Username: username},
		},
	}
	b, _ := json.Marshal(id)
	return base64.StdEncoding.EncodeToString(b)
}

func setupEchoWithKessel() (*echo.Echo, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()
	return e, rec
}

func callMiddleware(e *echo.Echo, rec *httptest.ResponseRecorder, mw echo.MiddlewareFunc, identityHeader string) (echo.Context, int, error) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if identityHeader != "" {
		req.Header.Set("X-Rh-Identity", identityHeader)
	}
	c := e.NewContext(req, rec)

	if identityHeader != "" {
		id := identity.XRHID{}
		decoded, _ := base64.StdEncoding.DecodeString(identityHeader)
		_ = json.Unmarshal(decoded, &id)
		c.Set("Identity", id)
	}

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "ok")
	}

	err := mw(handler)(c)
	code := rec.Code
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}
	if !handlerCalled && err == nil {
		code = rec.Code
	}

	return c, code, err
}

// UT-MW-AUTH-001: Full access (all 3 permissions allowed).
func TestKesselMiddlewareFullAccess(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
			"org-1|cost_management_openshift_node_read|user-1":    true,
			"org-1|cost_management_openshift_project_read|user-1": true,
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-AUTH-001: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("UT-MW-AUTH-001: expected HTTP 200, got %d", code)
	}

	perms, ok := c.Get("user.permissions").(map[string][]string)
	if !ok {
		t.Fatal("UT-MW-AUTH-001: user.permissions not set or wrong type")
	}
	if len(perms) != 3 {
		t.Errorf("UT-MW-AUTH-001: expected 3 permissions, got %d: %v", len(perms), perms)
	}
	for _, key := range []string{"openshift.cluster", "openshift.node", "openshift.project"} {
		vals, exists := perms[key]
		if !exists {
			t.Errorf("UT-MW-AUTH-001: missing permission %q", key)
			continue
		}
		if len(vals) != 1 || vals[0] != "*" {
			t.Errorf("UT-MW-AUTH-001: permission %q = %v, want [\"*\"]", key, vals)
		}
	}
}

// UT-MW-AUTH-002: Partial access (cluster only).
func TestKesselMiddlewarePartialAccess(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-AUTH-002: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("UT-MW-AUTH-002: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 1 {
		t.Errorf("UT-MW-AUTH-002: expected 1 permission, got %d: %v", len(perms), perms)
	}
	if _, exists := perms["openshift.cluster"]; !exists {
		t.Error("UT-MW-AUTH-002: expected openshift.cluster permission")
	}
}

// UT-MW-AUTH-003: Node + project access (2 of 3).
func TestKesselMiddlewareNodeProjectAccess(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_node_read|user-1":    true,
			"org-1|cost_management_openshift_project_read|user-1": true,
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-AUTH-003: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("UT-MW-AUTH-003: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 2 {
		t.Errorf("UT-MW-AUTH-003: expected 2 permissions, got %d: %v", len(perms), perms)
	}
}

// UT-MW-AUTH-004: No permissions -> HTTP 403.
func TestKesselMiddlewareNoPermissions(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-2"))

	if code != http.StatusForbidden {
		t.Errorf("UT-MW-AUTH-004: expected HTTP 403, got %d", code)
	}
}

// UT-MW-AUTH-005: Missing identity -> HTTP 401.
func TestKesselMiddlewareMissingIdentity(t *testing.T) {
	mock := &inputSensitiveMock{}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, "")

	if code != http.StatusUnauthorized {
		t.Errorf("UT-MW-AUTH-005: expected HTTP 401, got %d", code)
	}
}

// UT-MW-AUTH-006: Empty org ID -> HTTP 401.
func TestKesselMiddlewareEmptyOrgID(t *testing.T) {
	mock := &inputSensitiveMock{}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("", "user-1"))

	if code != http.StatusUnauthorized {
		t.Errorf("UT-MW-AUTH-006: expected HTTP 401, got %d", code)
	}
}

// UT-MW-AUTH-007: Empty username -> HTTP 401.
func TestKesselMiddlewareEmptyUsername(t *testing.T) {
	mock := &inputSensitiveMock{}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", ""))

	if code != http.StatusUnauthorized {
		t.Errorf("UT-MW-AUTH-007: expected HTTP 401, got %d", code)
	}
}

// UT-MW-AUTH-008: Partial gRPC failure (1 of 3 errors) -> remaining permissions set.
func TestKesselMiddlewarePartialGRPCFailure(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
			"org-1|cost_management_openshift_project_read|user-1": true,
		},
		errorTuples: map[string]error{
			"org-1|cost_management_openshift_node_read|user-1": status.Errorf(codes.Unavailable, "node check failed"),
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-AUTH-008: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("UT-MW-AUTH-008: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 2 {
		t.Errorf("UT-MW-AUTH-008: expected 2 permissions (node errored), got %d: %v", len(perms), perms)
	}
	if _, exists := perms["openshift.node"]; exists {
		t.Error("UT-MW-AUTH-008: openshift.node should not be present when its gRPC call failed")
	}
}

// UT-MW-AUTH-009: Total gRPC failure -> HTTP 424 (Failed Dependency).
// Distinguishes "Kessel is down" from "user has no permissions" (which is 403).
func TestKesselMiddlewareTotalGRPCFailure(t *testing.T) {
	grpcErr := status.Errorf(codes.Unavailable, "connection refused")
	mock := &inputSensitiveMock{
		errorTuples: map[string]error{
			"org-1|cost_management_openshift_cluster_read|user-1": grpcErr,
			"org-1|cost_management_openshift_node_read|user-1":    grpcErr,
			"org-1|cost_management_openshift_project_read|user-1": grpcErr,
		},
		listErrors: map[string]error{
			"org-1|cost_management/openshift_cluster|read|user-1": grpcErr,
			"org-1|cost_management/openshift_project|read|user-1": grpcErr,
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if code != http.StatusFailedDependency {
		t.Errorf("UT-MW-AUTH-009: expected HTTP 424 (Failed Dependency), got %d", code)
	}
}

// UT-MW-AUTH-010: No permissions (all denied, no errors) -> HTTP 403.
// Ensures genuine "no access" is still 403, distinct from total failure (424).
func TestKesselMiddlewareNoDenialVsTotalFailure(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {},
			"org-1|cost_management/openshift_project|read|user-1": {},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-denied"))

	if code != http.StatusForbidden {
		t.Errorf("UT-MW-AUTH-010: expected HTTP 403 for genuine no-access, got %d", code)
	}
}

// UT-MW-BACKEND-001: SelectAuthMiddleware dispatches correctly.
func TestSelectAuthMiddleware(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
			"org-1|cost_management_openshift_node_read|user-1":    true,
			"org-1|cost_management_openshift_project_read|user-1": true,
		},
	}

	tests := []struct {
		name     string
		rbacOn   bool
		backend  string
		wantNil  bool
		wantType string // "kessel" or "rbac"
	}{
		{
			name:     "RBACEnabled=false -> nil (no auth)",
			rbacOn:   false,
			backend:  "rbac",
			wantNil:  true,
			wantType: "",
		},
		{
			name:     "RBACEnabled=true, backend=rbac -> RBAC middleware",
			rbacOn:   true,
			backend:  "rbac",
			wantNil:  false,
			wantType: "rbac",
		},
		{
			name:     "RBACEnabled=true, backend=kessel -> Kessel middleware",
			rbacOn:   true,
			backend:  "kessel",
			wantNil:  false,
			wantType: "kessel",
		},
		{
			name:     "RBACEnabled=true, backend=unrecognized -> defaults to RBAC",
			rbacOn:   true,
			backend:  "foobar",
			wantNil:  false,
			wantType: "rbac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			config.ResetConfig()
			defer func() {
				viper.Reset()
				config.ResetConfig()
			}()

			testCfg := &config.Config{}
			testCfg.RBACEnabled = tt.rbacOn
			testCfg.AuthorizationBackend = tt.backend

			mw := SelectAuthMiddleware(testCfg, mock)
			if tt.wantNil {
				if mw != nil {
					t.Errorf("UT-MW-BACKEND-001 (%s): expected nil middleware, got non-nil", tt.name)
				}
				return
			}

			if mw == nil {
				t.Fatalf("UT-MW-BACKEND-001 (%s): expected non-nil middleware, got nil", tt.name)
			}

			// Verify Kessel middleware returns 200 when all permissions allowed
			if tt.wantType == "kessel" {
				e := echo.New()
				rec := httptest.NewRecorder()
				_, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))
				if err != nil {
					t.Fatalf("UT-MW-BACKEND-001 (%s): unexpected error: %v", tt.name, err)
				}
				if code != http.StatusOK {
					t.Errorf("UT-MW-BACKEND-001 (%s): expected HTTP 200 from Kessel middleware, got %d", tt.name, code)
				}
			}
			// For RBAC type, just verify it's not nil (RBAC calls real HTTP, tested in step 6)
		})
	}
}

// --- LookupResources + Check Fallback middleware tests (UT-MW-SLO-*) ---.
// These tests verify the middleware uses ListAuthorizedResources for.
// openshift.cluster and openshift.project, with Check fallback for wildcard,.
// and CheckPermission only for openshift.node.

// UT-MW-SLO-001: ListAuthorizedResources returns specific cluster IDs -> permissions contain those IDs.
func TestKesselMiddlewareSLOClusterSpecificIDs(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_node_read|user-1": true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {"cluster-a", "cluster-b"},
			"org-1|cost_management/openshift_project|read|user-1": {"proj-x"},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-001: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-001: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	clusterIDs := perms["openshift.cluster"]
	if len(clusterIDs) != 2 || clusterIDs[0] != "cluster-a" || clusterIDs[1] != "cluster-b" {
		t.Errorf("UT-MW-SLO-001: openshift.cluster = %v, want [cluster-a cluster-b]", clusterIDs)
	}
}

// UT-MW-SLO-002: ListAuthorizedResources returns specific project IDs.
func TestKesselMiddlewareSLOProjectSpecificIDs(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_node_read|user-1": true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {"cluster-a"},
			"org-1|cost_management/openshift_project|read|user-1": {"proj-x", "proj-y", "proj-z"},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-002: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-002: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	projectIDs := perms["openshift.project"]
	if len(projectIDs) != 3 {
		t.Errorf("UT-MW-SLO-002: openshift.project = %v, want [proj-x proj-y proj-z]", projectIDs)
	}
}

// UT-MW-SLO-003: ListAuthorizedResources returns empty, Check returns true -> wildcard ["*"].
func TestKesselMiddlewareSLOFallbackToWildcard(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
			"org-1|cost_management_openshift_project_read|user-1": true,
			"org-1|cost_management_openshift_node_read|user-1":    true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {},
			"org-1|cost_management/openshift_project|read|user-1": {},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-003: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-003: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	for _, key := range []string{"openshift.cluster", "openshift.project"} {
		vals := perms[key]
		if len(vals) != 1 || vals[0] != "*" {
			t.Errorf("UT-MW-SLO-003: %s = %v, want [\"*\"] (fallback wildcard)", key, vals)
		}
	}
}

// UT-MW-SLO-004: ListAuthorizedResources returns empty + Check returns false -> no permission.
func TestKesselMiddlewareSLONoAccessNoFallback(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {},
			"org-1|cost_management/openshift_project|read|user-1": {},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if code != http.StatusForbidden {
		t.Errorf("UT-MW-SLO-004: expected HTTP 403, got %d", code)
	}
}

// UT-MW-SLO-005: Node always uses CheckPermission (not ListAuthorizedResources) -> ["*"].
func TestKesselMiddlewareSLONodeAlwaysCheck(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_node_read|user-1": true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {"cluster-a"},
			"org-1|cost_management/openshift_project|read|user-1": {"proj-x"},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-005: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-005: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	nodeVals := perms["openshift.node"]
	if len(nodeVals) != 1 || nodeVals[0] != "*" {
		t.Errorf("UT-MW-SLO-005: openshift.node = %v, want [\"*\"]", nodeVals)
	}
}

// UT-MW-SLO-006: ListAuthorizedResources error, Check fallback succeeds -> wildcard.
func TestKesselMiddlewareSLOListErrorCheckFallback(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|user-1": true,
			"org-1|cost_management_openshift_node_read|user-1":    true,
		},
		listErrors: map[string]error{
			"org-1|cost_management/openshift_cluster|read|user-1": status.Errorf(codes.Unavailable, "list error"),
			"org-1|cost_management/openshift_project|read|user-1": status.Errorf(codes.Unavailable, "list error"),
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-006: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-006: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	clusterVals := perms["openshift.cluster"]
	if len(clusterVals) != 1 || clusterVals[0] != "*" {
		t.Errorf("UT-MW-SLO-006: openshift.cluster = %v, want [\"*\"] (Check fallback after List error)", clusterVals)
	}
}

// UT-MW-SLO-007: ListAuthorizedResources error + Check error -> no permission for that type.
func TestKesselMiddlewareSLOListErrorCheckError(t *testing.T) {
	mock := &inputSensitiveMock{
		errorTuples: map[string]error{
			"org-1|cost_management_openshift_cluster_read|user-1": status.Errorf(codes.Unavailable, "check error"),
		},
		listErrors: map[string]error{
			"org-1|cost_management/openshift_cluster|read|user-1": status.Errorf(codes.Unavailable, "list error"),
			"org-1|cost_management/openshift_project|read|user-1": status.Errorf(codes.Unavailable, "list error"),
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	_, code, _ := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if code != http.StatusForbidden {
		t.Errorf("UT-MW-SLO-007: expected HTTP 403, got %d", code)
	}
}

// UT-MW-SLO-008: Mixed scenario - cluster has IDs, project has wildcard fallback, node checked.
func TestKesselMiddlewareSLOMixed(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_project_read|user-1": true,
			"org-1|cost_management_openshift_node_read|user-1":    true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {"cluster-x"},
			"org-1|cost_management/openshift_project|read|user-1": {},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-008: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-008: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if clusterIDs := perms["openshift.cluster"]; len(clusterIDs) != 1 || clusterIDs[0] != "cluster-x" {
		t.Errorf("UT-MW-SLO-008: openshift.cluster = %v, want [cluster-x]", clusterIDs)
	}
	if projectVals := perms["openshift.project"]; len(projectVals) != 1 || projectVals[0] != "*" {
		t.Errorf("UT-MW-SLO-008: openshift.project = %v, want [\"*\"] (fallback)", projectVals)
	}
	if nodeVals := perms["openshift.node"]; len(nodeVals) != 1 || nodeVals[0] != "*" {
		t.Errorf("UT-MW-SLO-008: openshift.node = %v, want [\"*\"]", nodeVals)
	}
}

// UT-MW-SLO-009: Only node allowed, cluster/project denied -> 1 permission only.
func TestKesselMiddlewareSLONodeOnly(t *testing.T) {
	mock := &inputSensitiveMock{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_node_read|user-1": true,
		},
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {},
			"org-1|cost_management/openshift_project|read|user-1": {},
		},
	}

	e, rec := setupEchoWithKessel()
	mw := KesselMiddleware(mock)
	c, code, err := callMiddleware(e, rec, mw, encodeIdentity("org-1", "user-1"))

	if err != nil {
		t.Fatalf("UT-MW-SLO-009: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("UT-MW-SLO-009: expected HTTP 200, got %d", code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 1 {
		t.Errorf("UT-MW-SLO-009: expected 1 permission (node only), got %d: %v", len(perms), perms)
	}
	if _, exists := perms["openshift.node"]; !exists {
		t.Error("UT-MW-SLO-009: expected openshift.node permission")
	}
}
