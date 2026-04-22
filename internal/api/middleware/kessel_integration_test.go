//go:build integration

package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kessel"
	"github.com/redhatinsights/ros-ocp-backend/internal/testutil"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	kesselEndpoint = "localhost:9000"
)

var (
	kesselChecker kessel.PermissionChecker
	itSeeder      *testutil.KesselSeeder
)

func TestMain(m *testing.M) {
	var err error
	itSeeder, err = testutil.NewKesselSeeder(kesselEndpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to Kessel for IT: %v\n", err)
		os.Exit(1)
	}

	if err := seedTestData(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed IT test data: %v\n", err)
		os.Exit(1)
	}

	conn, err := grpc.NewClient(kesselEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to Kessel: %v\n", err)
		os.Exit(1)
	}
	kesselChecker = kessel.NewKesselClient(
		v1beta1.NewKesselCheckServiceClient(conn),
		v1beta1.NewKesselLookupServiceClient(conn),
	)

	code := m.Run()
	conn.Close()
	itSeeder.Close()
	os.Exit(code)
}

func seedTestData() error {
	ctx := context.Background()

	rels := []*v1beta1.Relationship{
		// Role with all 3 permissions: role-all-3
		testutil.Rel("rbac", "role", "role-all-3", "t_cost_management_openshift_cluster_read", "rbac", "principal", "user-1"),
		testutil.Rel("rbac", "role", "role-all-3", "t_cost_management_openshift_node_read", "rbac", "principal", "user-1"),
		testutil.Rel("rbac", "role", "role-all-3", "t_cost_management_openshift_project_read", "rbac", "principal", "user-1"),

		// Binding for user-1 in org-1: all 3 permissions
		testutil.Rel("rbac", "role_binding", "binding-1", "t_granted", "rbac", "role", "role-all-3"),
		testutil.Rel("rbac", "role_binding", "binding-1", "t_subject", "rbac", "principal", "user-1"),
		testutil.Rel("rbac", "tenant", "org-1", "t_default_binding", "rbac", "role_binding", "binding-1"),

		// Workspace + resources for LookupResources (user-1 in org-1)
		testutil.Rel("rbac", "workspace", "ws-org1", "t_parent", "rbac", "tenant", "org-1"),
		testutil.Rel("rbac", "workspace", "ws-org1", "t_binding", "rbac", "role_binding", "binding-1"),
		testutil.Rel("cost_management", "openshift_cluster", "it-cluster-a", "t_workspace", "rbac", "workspace", "ws-org1"),
		testutil.Rel("cost_management", "openshift_cluster", "it-cluster-b", "t_workspace", "rbac", "workspace", "ws-org1"),
		testutil.Rel("cost_management", "openshift_project", "it-proj-x", "t_workspace", "rbac", "workspace", "ws-org1"),
		testutil.Rel("cost_management", "openshift_project", "it-proj-y", "t_workspace", "rbac", "workspace", "ws-org1"),

		// user-2 in org-1: no bindings (tested via absence)

		// user-3 in org-1: cluster only
		testutil.Rel("rbac", "role", "role-cluster-only", "t_cost_management_openshift_cluster_read", "rbac", "principal", "user-3"),
		testutil.Rel("rbac", "role_binding", "binding-3", "t_granted", "rbac", "role", "role-cluster-only"),
		testutil.Rel("rbac", "role_binding", "binding-3", "t_subject", "rbac", "principal", "user-3"),
		testutil.Rel("rbac", "tenant", "org-1", "t_default_binding", "rbac", "role_binding", "binding-3"),

		// user-4 in org-1: member of group-1 with all 3 perms
		testutil.Rel("rbac", "group", "group-1", "member", "rbac", "principal", "user-4"),
		testutil.RelWithSubRelation("rbac", "role", "role-group-all", "t_cost_management_openshift_cluster_read", "rbac", "group", "group-1", "member"),
		testutil.RelWithSubRelation("rbac", "role", "role-group-all", "t_cost_management_openshift_node_read", "rbac", "group", "group-1", "member"),
		testutil.RelWithSubRelation("rbac", "role", "role-group-all", "t_cost_management_openshift_project_read", "rbac", "group", "group-1", "member"),
		testutil.Rel("rbac", "role_binding", "binding-4", "t_granted", "rbac", "role", "role-group-all"),
		testutil.RelWithSubRelation("rbac", "role_binding", "binding-4", "t_subject", "rbac", "group", "group-1", "member"),
		testutil.Rel("rbac", "tenant", "org-1", "t_default_binding", "rbac", "role_binding", "binding-4"),

		// user-5 in org-2: all 3 perms (cross-tenant test)
		testutil.Rel("rbac", "role", "role-org2-all", "t_cost_management_openshift_cluster_read", "rbac", "principal", "user-5"),
		testutil.Rel("rbac", "role", "role-org2-all", "t_cost_management_openshift_node_read", "rbac", "principal", "user-5"),
		testutil.Rel("rbac", "role", "role-org2-all", "t_cost_management_openshift_project_read", "rbac", "principal", "user-5"),
		testutil.Rel("rbac", "role_binding", "binding-5", "t_granted", "rbac", "role", "role-org2-all"),
		testutil.Rel("rbac", "role_binding", "binding-5", "t_subject", "rbac", "principal", "user-5"),
		testutil.Rel("rbac", "tenant", "org-2", "t_default_binding", "rbac", "role_binding", "binding-5"),

		// user-6 and user-7 in org-3: E2E handler tests
		testutil.Rel("rbac", "role", "role-org3-all", "t_cost_management_openshift_cluster_read", "rbac", "principal", "user-6"),
		testutil.Rel("rbac", "role", "role-org3-all", "t_cost_management_openshift_node_read", "rbac", "principal", "user-6"),
		testutil.Rel("rbac", "role", "role-org3-all", "t_cost_management_openshift_project_read", "rbac", "principal", "user-6"),
		testutil.Rel("rbac", "role_binding", "binding-6", "t_granted", "rbac", "role", "role-org3-all"),
		testutil.Rel("rbac", "role_binding", "binding-6", "t_subject", "rbac", "principal", "user-6"),
		testutil.Rel("rbac", "tenant", "org-3", "t_default_binding", "rbac", "role_binding", "binding-6"),
		// user-7: no bindings in org-3

		// user-9 in org-5: for backend switch test
		testutil.Rel("rbac", "role", "role-org5-all", "t_cost_management_openshift_cluster_read", "rbac", "principal", "user-9"),
		testutil.Rel("rbac", "role", "role-org5-all", "t_cost_management_openshift_node_read", "rbac", "principal", "user-9"),
		testutil.Rel("rbac", "role", "role-org5-all", "t_cost_management_openshift_project_read", "rbac", "principal", "user-9"),
		testutil.Rel("rbac", "role_binding", "binding-9", "t_granted", "rbac", "role", "role-org5-all"),
		testutil.Rel("rbac", "role_binding", "binding-9", "t_subject", "rbac", "principal", "user-9"),
		testutil.Rel("rbac", "tenant", "org-5", "t_default_binding", "rbac", "role_binding", "binding-9"),
	}

	if err := itSeeder.SeedTuples(ctx, rels); err != nil {
		return fmt.Errorf("seeding IT relationships: %w", err)
	}

	if err := itSeeder.WaitForConsistency(ctx, "org-1", "cost_management_openshift_cluster_read", "user-1", true, 30*time.Second); err != nil {
		return fmt.Errorf("waiting for consistency: %w", err)
	}

	return nil
}

func itEncodeIdentity(orgID, username string) string {
	id := identity.XRHID{
		Identity: identity.Identity{
			OrgID: orgID,
			User:  identity.User{Username: username},
		},
	}
	b, _ := json.Marshal(id)
	return base64.StdEncoding.EncodeToString(b)
}

func itCallMiddleware(t *testing.T, mw echo.MiddlewareFunc, orgID, username string) (*httptest.ResponseRecorder, echo.Context) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	hdr := itEncodeIdentity(orgID, username)
	req.Header.Set("X-Rh-Identity", hdr)
	c := e.NewContext(req, rec)

	id := identity.XRHID{}
	decoded, _ := base64.StdEncoding.DecodeString(hdr)
	_ = json.Unmarshal(decoded, &id)
	c.Set("Identity", id)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "ok")
	}

	err := mw(handler)(c)
	if he, ok := err.(*echo.HTTPError); ok {
		rec.Code = he.Code
	} else if err == nil && handlerCalled {
		rec.Code = http.StatusOK
	}

	return rec, c
}

// IT-MW-AUTH-001: user-1/org-1 -> 3 permissions, HTTP 200.
func TestITFullAccess(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-AUTH-001: expected HTTP 200, got %d", rec.Code)
	}
	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 3 {
		t.Errorf("IT-MW-AUTH-001: expected 3 permissions, got %d: %v", len(perms), perms)
	}
}

// IT-MW-AUTH-002: user-2/org-1 -> HTTP 403.
func TestITNoPermissions(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-1", "user-2")

	if rec.Code != http.StatusForbidden {
		t.Errorf("IT-MW-AUTH-002: expected HTTP 403, got %d", rec.Code)
	}
}

// IT-MW-AUTH-003: user-3/org-1 -> cluster only.
func TestITPartialPermissions(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-3")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-AUTH-003: expected HTTP 200, got %d", rec.Code)
	}
	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 1 {
		t.Errorf("IT-MW-AUTH-003: expected 1 permission, got %d: %v", len(perms), perms)
	}
	if _, exists := perms["openshift.cluster"]; !exists {
		t.Error("IT-MW-AUTH-003: expected openshift.cluster permission")
	}
}

// IT-MW-AUTH-004: user-4/org-1 -> 3 permissions via group.
func TestITGroupAccess(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-4")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-AUTH-004: expected HTTP 200, got %d", rec.Code)
	}
	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 3 {
		t.Errorf("IT-MW-AUTH-004: expected 3 permissions (via group), got %d: %v", len(perms), perms)
	}
}

// IT-MW-AUTH-005: user-5/org-1 -> HTTP 403 (cross-tenant).
func TestITCrossTenantDenied(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-1", "user-5")

	if rec.Code != http.StatusForbidden {
		t.Errorf("IT-MW-AUTH-005: expected HTTP 403 for cross-tenant, got %d", rec.Code)
	}
}

// IT-MW-AUTH-006: user-6/org-3 -> HTTP 200.
func TestITE2EAuthorized(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-3", "user-6")

	if rec.Code != http.StatusOK {
		t.Errorf("IT-MW-AUTH-006: expected HTTP 200, got %d", rec.Code)
	}
}

// IT-MW-AUTH-007: user-7/org-3 -> HTTP 403.
func TestITE2EUnauthorized(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-3", "user-7")

	if rec.Code != http.StatusForbidden {
		t.Errorf("IT-MW-AUTH-007: expected HTTP 403, got %d", rec.Code)
	}
}

// IT-MW-AUTH-008: Sequential requests user-6 (200) then user-7 (403).
func TestITSequentialRequests(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)

	rec1, _ := itCallMiddleware(t, mw, "org-3", "user-6")
	if rec1.Code != http.StatusOK {
		t.Errorf("IT-MW-AUTH-008a: expected HTTP 200 for user-6, got %d", rec1.Code)
	}

	rec2, _ := itCallMiddleware(t, mw, "org-3", "user-7")
	if rec2.Code != http.StatusForbidden {
		t.Errorf("IT-MW-AUTH-008b: expected HTTP 403 for user-7, got %d", rec2.Code)
	}
}

// IT-MW-BACKEND-001: Kessel config -> 200; RBAC config -> uses RBAC middleware.
func TestITBackendSwitch(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-5", "user-9")
	if rec.Code != http.StatusOK {
		t.Errorf("IT-MW-BACKEND-001 (kessel): expected HTTP 200, got %d", rec.Code)
	}

	viper.Reset()
	config.ResetConfig()
	t.Cleanup(func() { viper.Reset(); config.ResetConfig() })

	testCfg := &config.Config{}
	testCfg.RBACEnabled = true
	testCfg.AuthorizationBackend = "rbac"
	rbacMW := SelectAuthMiddleware(testCfg, kesselChecker)
	if rbacMW == nil {
		t.Fatal("IT-MW-BACKEND-001 (rbac): SelectAuthMiddleware returned nil for RBAC config")
	}
}

// --- SLO (LookupResources) integration tests ---.

// IT-MW-SLO-001: user-1/org-1 gets specific cluster IDs via LookupResources.
func TestITSLOClusterIDs(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-SLO-001: expected HTTP 200, got %d", rec.Code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	clusterIDs := perms["openshift.cluster"]
	if len(clusterIDs) < 2 {
		t.Errorf("IT-MW-SLO-001: expected >=2 cluster IDs, got %d: %v", len(clusterIDs), clusterIDs)
	}
}

// IT-MW-SLO-002: user-1/org-1 gets specific project IDs via LookupResources.
func TestITSLOProjectIDs(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-SLO-002: expected HTTP 200, got %d", rec.Code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	projectIDs := perms["openshift.project"]
	if len(projectIDs) < 2 {
		t.Errorf("IT-MW-SLO-002: expected >=2 project IDs, got %d: %v", len(projectIDs), projectIDs)
	}
}

// IT-MW-SLO-003: user-1/org-1 node is always wildcard (no LookupResources for node).
func TestITSLONodeWildcard(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-SLO-003: expected HTTP 200, got %d", rec.Code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	nodeVals := perms["openshift.node"]
	if len(nodeVals) != 1 || nodeVals[0] != "*" {
		t.Errorf("IT-MW-SLO-003: openshift.node = %v, want [\"*\"]", nodeVals)
	}
}

// IT-MW-SLO-004: user-2/org-1 (no bindings) -> no LookupResources results, no Check fallback -> 403.
func TestITSLONoAccess(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, _ := itCallMiddleware(t, mw, "org-1", "user-2")

	if rec.Code != http.StatusForbidden {
		t.Errorf("IT-MW-SLO-004: expected HTTP 403, got %d", rec.Code)
	}
}

// itAssertExactIDs verifies that got contains exactly the expected IDs (order-independent, no wildcards).
func itAssertExactIDs(t *testing.T, testID, permKey string, got []string, want []string) {
	t.Helper()
	wantSet := make(map[string]bool, len(want))
	for _, id := range want {
		wantSet[id] = false
	}
	if len(got) != len(wantSet) {
		t.Errorf("%s: %s = %v, want %d specific IDs", testID, permKey, got, len(wantSet))
	}
	for _, id := range got {
		if id == "*" {
			t.Errorf("%s: %s contains wildcard; expected specific IDs from LookupResources", testID, permKey)
		}
		if _, expected := wantSet[id]; !expected {
			t.Errorf("%s: unexpected %s ID %q", testID, permKey, id)
		}
		wantSet[id] = true
	}
	for id, found := range wantSet {
		if !found {
			t.Errorf("%s: missing expected %s ID %q", testID, permKey, id)
		}
	}
}

// IT-MW-COMBINED-001: Single request produces specific cluster IDs + specific project IDs + wildcard node.
func TestITCombinedClusterProjectNode(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-COMBINED-001: expected HTTP 200, got %d", rec.Code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	if len(perms) != 3 {
		t.Fatalf("IT-MW-COMBINED-001: expected 3 permission keys, got %d: %v", len(perms), perms)
	}

	itAssertExactIDs(t, "IT-MW-COMBINED-001", "openshift.cluster", perms["openshift.cluster"], []string{"it-cluster-a", "it-cluster-b"})
	itAssertExactIDs(t, "IT-MW-COMBINED-001", "openshift.project", perms["openshift.project"], []string{"it-proj-x", "it-proj-y"})

	nodeVals := perms["openshift.node"]
	if len(nodeVals) != 1 || nodeVals[0] != "*" {
		t.Errorf("IT-MW-COMBINED-001: openshift.node = %v, want [\"*\"]", nodeVals)
	}
}

// IT-MW-PARITY-001: RBAC-style and Kessel-style permissions have the same map structure.
func TestITPermissionShapeParity(t *testing.T) {
	mw := KesselMiddleware(kesselChecker)
	rec, c := itCallMiddleware(t, mw, "org-1", "user-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("IT-MW-PARITY-001: expected HTTP 200, got %d", rec.Code)
	}

	perms := c.Get("user.permissions").(map[string][]string)
	expectedKeys := []string{"openshift.cluster", "openshift.node", "openshift.project"}
	for _, key := range expectedKeys {
		vals, exists := perms[key]
		if !exists {
			t.Errorf("IT-MW-PARITY-001: missing key %q in permissions", key)
			continue
		}
		if len(vals) == 0 {
			t.Errorf("IT-MW-PARITY-001: %q has empty values", key)
		}
		// Each value is either "*" or a list of specific IDs (both valid shapes)
		for _, v := range vals {
			if v == "" {
				t.Errorf("IT-MW-PARITY-001: %q contains empty string value", key)
			}
		}
	}
}
