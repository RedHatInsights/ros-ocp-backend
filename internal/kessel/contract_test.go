//go:build contract

package kessel

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"time"

	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/redhatinsights/ros-ocp-backend/internal/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ctKesselEndpoint = "localhost:9000"
)

var (
	seeder       *testutil.KesselSeeder
	lookupClient v1beta1.KesselLookupServiceClient
)

func TestMain(m *testing.M) {
	var err error
	seeder, err = testutil.NewKesselSeeder(ctKesselEndpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to Kessel for CT: %v\n", err)
		os.Exit(1)
	}

	conn, err := grpc.NewClient(ctKesselEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lookup client: %v\n", err)
		os.Exit(1)
	}
	lookupClient = v1beta1.NewKesselLookupServiceClient(conn)

	if err := seedCTData(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed CT test data: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	conn.Close()
	seeder.Close()
	os.Exit(code)
}

func seedCTData() error {
	ctx := context.Background()

	rels := []*v1beta1.Relationship{
		// --- RBAC hierarchy for ct-user-1 in org-ct1: all 3 permissions ---
		testutil.Rel("rbac", "role", "ct-role-1", "t_cost_management_openshift_cluster_read", "rbac", "principal", "ct-user-1"),
		testutil.Rel("rbac", "role", "ct-role-1", "t_cost_management_openshift_node_read", "rbac", "principal", "ct-user-1"),
		testutil.Rel("rbac", "role", "ct-role-1", "t_cost_management_openshift_project_read", "rbac", "principal", "ct-user-1"),
		testutil.Rel("rbac", "role_binding", "ct-binding-1", "t_granted", "rbac", "role", "ct-role-1"),
		testutil.Rel("rbac", "role_binding", "ct-binding-1", "t_subject", "rbac", "principal", "ct-user-1"),
		testutil.Rel("rbac", "tenant", "org-ct1", "t_default_binding", "rbac", "role_binding", "ct-binding-1"),

		// --- Workspace + resources for LookupResources ---
		testutil.Rel("rbac", "workspace", "ws-ct1", "t_parent", "rbac", "tenant", "org-ct1"),
		testutil.Rel("rbac", "workspace", "ws-ct1", "t_binding", "rbac", "role_binding", "ct-binding-1"),
		testutil.Rel("cost_management", "openshift_cluster", "cluster-ct-a", "t_workspace", "rbac", "workspace", "ws-ct1"),
		testutil.Rel("cost_management", "openshift_cluster", "cluster-ct-b", "t_workspace", "rbac", "workspace", "ws-ct1"),
		testutil.Rel("cost_management", "openshift_project", "proj-ct-x", "t_workspace", "rbac", "workspace", "ws-ct1"),

		// ct-user-2 in org-ct1: no bindings

		// Cluster-only role for permission isolation test
		testutil.Rel("rbac", "role", "ct-role-cluster-only", "t_cost_management_openshift_cluster_read", "rbac", "principal", "ct-user-iso"),
		testutil.Rel("rbac", "role_binding", "ct-binding-iso", "t_granted", "rbac", "role", "ct-role-cluster-only"),
		testutil.Rel("rbac", "role_binding", "ct-binding-iso", "t_subject", "rbac", "principal", "ct-user-iso"),
		testutil.Rel("rbac", "tenant", "org-ct1", "t_default_binding", "rbac", "role_binding", "ct-binding-iso"),

		// ct-user-3 via group-ct1
		testutil.Rel("rbac", "group", "group-ct1", "member", "rbac", "principal", "ct-user-3"),
		testutil.RelWithSubRelation("rbac", "role", "ct-role-grp", "t_cost_management_openshift_cluster_read", "rbac", "group", "group-ct1", "member"),
		testutil.RelWithSubRelation("rbac", "role", "ct-role-grp", "t_cost_management_openshift_node_read", "rbac", "group", "group-ct1", "member"),
		testutil.RelWithSubRelation("rbac", "role", "ct-role-grp", "t_cost_management_openshift_project_read", "rbac", "group", "group-ct1", "member"),
		testutil.Rel("rbac", "role_binding", "ct-binding-grp", "t_granted", "rbac", "role", "ct-role-grp"),
		testutil.RelWithSubRelation("rbac", "role_binding", "ct-binding-grp", "t_subject", "rbac", "group", "group-ct1", "member"),
		testutil.Rel("rbac", "tenant", "org-ct1", "t_default_binding", "rbac", "role_binding", "ct-binding-grp"),
	}

	if err := seeder.SeedTuples(ctx, rels); err != nil {
		return fmt.Errorf("seeding CT relationships: %w", err)
	}

	if err := seeder.WaitForConsistency(ctx, "org-ct1", "cost_management_openshift_cluster_read", "ct-user-1", true, 30*time.Second); err != nil {
		return fmt.Errorf("waiting for consistency: %w", err)
	}

	return nil
}

func ctCheck(t *testing.T, orgID, permission, username string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	allowed, err := seeder.Check(ctx, orgID, permission, username)
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	return allowed
}

// ctLookupResources calls LookupResources and returns the resource IDs found.
func ctLookupResources(t *testing.T, resourceNs, resourceName, relation, subjectID string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := lookupClient.LookupResources(ctx, &v1beta1.LookupResourcesRequest{
		ResourceType: &v1beta1.ObjectType{Namespace: resourceNs, Name: resourceName},
		Relation:     relation,
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   subjectID,
			},
		},
	})
	if err != nil {
		t.Fatalf("LookupResources call failed: %v", err)
	}

	seen := make(map[string]struct{})
	var ids []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("LookupResources stream error: %v", err)
		}
		if resp.Resource != nil {
			if _, dup := seen[resp.Resource.Id]; !dup {
				seen[resp.Resource.Id] = struct{}{}
				ids = append(ids, resp.Resource.Id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// --- Check (tenant-level) contract tests ---.

// CT-KESSEL-SCHEMA-001: ct-user-1 in org-ct1 has cluster_read.
func TestCTSchemaClusterRead(t *testing.T) {
	if !ctCheck(t, "org-ct1", "cost_management_openshift_cluster_read", "ct-user-1") {
		t.Error("CT-KESSEL-SCHEMA-001: expected ALLOWED_TRUE for cluster_read")
	}
}

// CT-KESSEL-SCHEMA-002: ct-user-1 in org-ct1 has node_read.
func TestCTSchemaNodeRead(t *testing.T) {
	if !ctCheck(t, "org-ct1", "cost_management_openshift_node_read", "ct-user-1") {
		t.Error("CT-KESSEL-SCHEMA-002: expected ALLOWED_TRUE for node_read")
	}
}

// CT-KESSEL-SCHEMA-003: ct-user-1 in org-ct1 has project_read.
func TestCTSchemaProjectRead(t *testing.T) {
	if !ctCheck(t, "org-ct1", "cost_management_openshift_project_read", "ct-user-1") {
		t.Error("CT-KESSEL-SCHEMA-003: expected ALLOWED_TRUE for project_read")
	}
}

// CT-KESSEL-REL-001: ct-user-2 (unbound) -> ALLOWED_FALSE.
func TestCTUnboundUser(t *testing.T) {
	if ctCheck(t, "org-ct1", "cost_management_openshift_cluster_read", "ct-user-2") {
		t.Error("CT-KESSEL-REL-001: expected ALLOWED_FALSE for unbound user")
	}
}

// CT-KESSEL-REL-002: ct-user-iso with cluster-only binding, check node_read -> ALLOWED_FALSE.
func TestCTPermissionIsolation(t *testing.T) {
	if ctCheck(t, "org-ct1", "cost_management_openshift_node_read", "ct-user-iso") {
		t.Error("CT-KESSEL-REL-002: expected ALLOWED_FALSE for node_read with cluster-only binding")
	}
}

// CT-KESSEL-GRP-001: ct-user-3 (group member) -> ALLOWED_TRUE.
func TestCTGroupMember(t *testing.T) {
	if !ctCheck(t, "org-ct1", "cost_management_openshift_cluster_read", "ct-user-3") {
		t.Error("CT-KESSEL-GRP-001: expected ALLOWED_TRUE for group member")
	}
}

// CT-KESSEL-REL-003: ct-user-1 in org-ct2 -> ALLOWED_FALSE (cross-tenant).
func TestCTCrossTenant(t *testing.T) {
	if ctCheck(t, "org-ct2", "cost_management_openshift_cluster_read", "ct-user-1") {
		t.Error("CT-KESSEL-REL-003: expected ALLOWED_FALSE for cross-tenant check")
	}
}

// --- LookupResources contract tests ---.

// CT-KESSEL-SLO-001: LookupResources returns cluster IDs for ct-user-1.
func TestCTLookupClusterIDs(t *testing.T) {
	ids := ctLookupResources(t, "cost_management", "openshift_cluster", "read", "ct-user-1")
	if len(ids) != 2 {
		t.Fatalf("CT-KESSEL-SLO-001: expected 2 cluster IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "cluster-ct-a" || ids[1] != "cluster-ct-b" {
		t.Errorf("CT-KESSEL-SLO-001: expected [cluster-ct-a cluster-ct-b], got %v", ids)
	}
}

// CT-KESSEL-SLO-002: LookupResources returns project IDs for ct-user-1.
func TestCTLookupProjectIDs(t *testing.T) {
	ids := ctLookupResources(t, "cost_management", "openshift_project", "read", "ct-user-1")
	if len(ids) != 1 || ids[0] != "proj-ct-x" {
		t.Errorf("CT-KESSEL-SLO-002: expected [proj-ct-x], got %v", ids)
	}
}

// CT-KESSEL-SLO-003: LookupResources for unbound user returns empty.
func TestCTLookupUnboundUser(t *testing.T) {
	ids := ctLookupResources(t, "cost_management", "openshift_cluster", "read", "ct-user-2")
	if len(ids) != 0 {
		t.Errorf("CT-KESSEL-SLO-003: expected empty for unbound user, got %v", ids)
	}
}

// CT-KESSEL-SEED-001: CreateTuples idempotent upsert succeeds.
func TestCTSeedIdempotent(t *testing.T) {
	ctx := context.Background()
	rels := []*v1beta1.Relationship{
		testutil.Rel("rbac", "role", "ct-role-1", "t_cost_management_openshift_cluster_read", "rbac", "principal", "ct-user-1"),
	}
	if err := seeder.SeedTuples(ctx, rels); err != nil {
		t.Fatalf("CT-KESSEL-SEED-001: re-seeding should be idempotent, got error: %v", err)
	}
	if !ctCheck(t, "org-ct1", "cost_management_openshift_cluster_read", "ct-user-1") {
		t.Error("CT-KESSEL-SEED-001: expected ALLOWED_TRUE after re-seed")
	}
}
