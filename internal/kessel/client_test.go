package kessel

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockCheckService implements v1beta1.KesselCheckServiceClient with.
// input-sensitive behavior: it returns ALLOWED_TRUE only for explicitly.
// registered tuples and error responses for registered error tuples.
type mockCheckService struct {
	allowedTuples map[string]bool
	errorTuples   map[string]error
}

// mockLookupStream simulates a server-streaming gRPC response for LookupResources.
type mockLookupStream struct {
	responses []*v1beta1.LookupResourcesResponse
	idx       int
}

func (s *mockLookupStream) Recv() (*v1beta1.LookupResourcesResponse, error) {
	if s.idx >= len(s.responses) {
		return nil, io.EOF
	}
	resp := s.responses[s.idx]
	s.idx++
	return resp, nil
}

func (s *mockLookupStream) Header() (metadata.MD, error) { return nil, nil }
func (s *mockLookupStream) Trailer() metadata.MD         { return nil }
func (s *mockLookupStream) CloseSend() error             { return nil }
func (s *mockLookupStream) Context() context.Context     { return context.Background() }
func (s *mockLookupStream) SendMsg(any) error            { return nil }
func (s *mockLookupStream) RecvMsg(any) error            { return nil }

// mockLookupErrorStream returns an error on the first Recv (simulating a stream error).
type mockLookupErrorStream struct {
	err error
}

func (s *mockLookupErrorStream) Recv() (*v1beta1.LookupResourcesResponse, error) {
	return nil, s.err
}

func (s *mockLookupErrorStream) Header() (metadata.MD, error) { return nil, nil }
func (s *mockLookupErrorStream) Trailer() metadata.MD         { return nil }
func (s *mockLookupErrorStream) CloseSend() error             { return nil }
func (s *mockLookupErrorStream) Context() context.Context     { return context.Background() }
func (s *mockLookupErrorStream) SendMsg(any) error            { return nil }
func (s *mockLookupErrorStream) RecvMsg(any) error            { return nil }

// mockLookupClient implements KesselLookupServiceClient for unit tests.
type mockLookupClient struct {
	responses []*v1beta1.LookupResourcesResponse
	lookupErr error
	streamErr error
}

func (m *mockLookupClient) LookupResources(_ context.Context, _ *v1beta1.LookupResourcesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	if m.lookupErr != nil {
		return nil, m.lookupErr
	}
	if m.streamErr != nil {
		return &mockLookupErrorStream{err: m.streamErr}, nil
	}
	return &mockLookupStream{responses: m.responses}, nil
}

func (m *mockLookupClient) LookupSubjects(_ context.Context, _ *v1beta1.LookupSubjectsRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[v1beta1.LookupSubjectsResponse], error) {
	return nil, status.Errorf(codes.Unimplemented, "not used in tests")
}

func (m *mockCheckService) Check(ctx context.Context, in *v1beta1.CheckRequest, opts ...grpc.CallOption) (*v1beta1.CheckResponse, error) {
	key := fmt.Sprintf("%s|%s|%s", in.Resource.Id, in.Relation, in.Subject.Subject.Id)
	if err, ok := m.errorTuples[key]; ok {
		return nil, err
	}
	if m.allowedTuples[key] {
		return &v1beta1.CheckResponse{Allowed: v1beta1.CheckResponse_ALLOWED_TRUE}, nil
	}
	return &v1beta1.CheckResponse{Allowed: v1beta1.CheckResponse_ALLOWED_FALSE}, nil
}

func (m *mockCheckService) CheckForUpdate(ctx context.Context, in *v1beta1.CheckForUpdateRequest, opts ...grpc.CallOption) (*v1beta1.CheckForUpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not used in tests")
}

func (m *mockCheckService) CheckBulk(ctx context.Context, in *v1beta1.CheckBulkRequest, opts ...grpc.CallOption) (*v1beta1.CheckBulkResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not used in tests")
}

func TestCheckPermissionAllowed(t *testing.T) {
	mock := &mockCheckService{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|redhat/user-1": true,
		},
	}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "org-1", "cost_management_openshift_cluster_read", "user-1")
	if err != nil {
		t.Fatalf("UT-KESSEL-CHECK-001: unexpected error: %v", err)
	}
	if !allowed {
		t.Error("UT-KESSEL-CHECK-001: expected allowed=true for registered tuple, got false")
	}
}

func TestCheckPermissionDeniedDifferentUser(t *testing.T) {
	mock := &mockCheckService{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|redhat/user-1": true,
		},
	}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "org-1", "cost_management_openshift_cluster_read", "user-2")
	if err != nil {
		t.Fatalf("UT-KESSEL-CHECK-002: unexpected error: %v", err)
	}
	if allowed {
		t.Error("UT-KESSEL-CHECK-002: expected allowed=false for unregistered user, got true")
	}
}

func TestCheckPermissionDeniedDifferentOrg(t *testing.T) {
	mock := &mockCheckService{
		allowedTuples: map[string]bool{
			"org-1|cost_management_openshift_cluster_read|redhat/user-1": true,
		},
	}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "org-2", "cost_management_openshift_cluster_read", "user-1")
	if err != nil {
		t.Fatalf("UT-KESSEL-CHECK-003: unexpected error: %v", err)
	}
	if allowed {
		t.Error("UT-KESSEL-CHECK-003: expected allowed=false for different org, got true")
	}
}

func TestCheckPermissionGRPCUnavailable(t *testing.T) {
	mock := &mockCheckService{
		errorTuples: map[string]error{
			"org-1|cost_management_openshift_cluster_read|redhat/user-1": status.Errorf(codes.Unavailable, "connection refused"),
		},
	}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "org-1", "cost_management_openshift_cluster_read", "user-1")
	if err == nil {
		t.Fatal("UT-KESSEL-CHECK-004: expected error for gRPC unavailable, got nil")
	}
	if allowed {
		t.Error("UT-KESSEL-CHECK-004: expected allowed=false on error, got true")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("UT-KESSEL-CHECK-004: expected gRPC Unavailable status, got %v", err)
	}
}

func TestCheckPermissionEmptyOrgID(t *testing.T) {
	mock := &mockCheckService{}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "", "cost_management_openshift_cluster_read", "user-1")
	if err == nil {
		t.Fatal("UT-KESSEL-CHECK-005: expected error for empty org_id, got nil")
	}
	if !strings.Contains(err.Error(), "org_id") {
		t.Errorf("UT-KESSEL-CHECK-005: error %q should mention 'org_id'", err.Error())
	}
	if allowed {
		t.Error("UT-KESSEL-CHECK-005: expected allowed=false for empty org_id, got true")
	}
}

func TestCheckPermissionEmptyUsername(t *testing.T) {
	mock := &mockCheckService{}
	client := NewKesselClient(mock)

	allowed, err := client.CheckPermission(context.Background(), "org-1", "cost_management_openshift_cluster_read", "")
	if err == nil {
		t.Fatal("UT-KESSEL-CHECK-006: expected error for empty username, got nil")
	}
	if !strings.Contains(err.Error(), "username") {
		t.Errorf("UT-KESSEL-CHECK-006: error %q should mention 'username'", err.Error())
	}
	if allowed {
		t.Error("UT-KESSEL-CHECK-006: expected allowed=false for empty username, got true")
	}
}

// --- ListAuthorizedResources unit tests ---.

// UT-KESSEL-LIST-001: returns specific IDs for seeded resources.
func TestListAuthorizedResourcesReturnsIDs(t *testing.T) {
	checker := &mockPermissionChecker{
		authorizedIDs: map[string][]string{
			"org-1|cost_management/openshift_cluster|read|user-1": {"cluster-a", "cluster-b"},
		},
	}

	ids, err := checker.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err != nil {
		t.Fatalf("UT-KESSEL-LIST-001: unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("UT-KESSEL-LIST-001: expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "cluster-a" || ids[1] != "cluster-b" {
		t.Errorf("UT-KESSEL-LIST-001: expected [cluster-a cluster-b], got %v", ids)
	}
}

// UT-KESSEL-LIST-002: returns empty slice when no resource bindings exist.
func TestListAuthorizedResourcesEmpty(t *testing.T) {
	checker := &mockPermissionChecker{
		authorizedIDs: map[string][]string{},
	}

	ids, err := checker.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-no-access")
	if err != nil {
		t.Fatalf("UT-KESSEL-LIST-002: unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("UT-KESSEL-LIST-002: expected empty slice, got %v", ids)
	}
}

// UT-KESSEL-LIST-003: propagates gRPC Unavailable error.
func TestListAuthorizedResourcesGRPCError(t *testing.T) {
	checker := &mockPermissionChecker{
		listErrors: map[string]error{
			"org-1|cost_management/openshift_cluster|read|user-1": status.Errorf(codes.Unavailable, "connection refused"),
		},
	}

	ids, err := checker.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err == nil {
		t.Fatal("UT-KESSEL-LIST-003: expected error for gRPC unavailable, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("UT-KESSEL-LIST-003: expected gRPC Unavailable status, got %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("UT-KESSEL-LIST-003: expected empty slice on error, got %v", ids)
	}
}

// UT-KESSEL-LIST-004: empty orgID returns input validation error.
func TestListAuthorizedResourcesEmptyOrgID(t *testing.T) {
	checker := &mockPermissionChecker{}

	ids, err := checker.ListAuthorizedResources(context.Background(), "", "cost_management/openshift_cluster", "read", "user-1")
	if err == nil {
		t.Fatal("UT-KESSEL-LIST-004: expected error for empty org_id, got nil")
	}
	if !strings.Contains(err.Error(), "org_id") {
		t.Errorf("UT-KESSEL-LIST-004: error %q should mention 'org_id'", err.Error())
	}
	if len(ids) != 0 {
		t.Errorf("UT-KESSEL-LIST-004: expected empty slice on error, got %v", ids)
	}
}

// UT-KESSEL-LIST-005: empty username returns input validation error.
func TestListAuthorizedResourcesEmptyUsername(t *testing.T) {
	checker := &mockPermissionChecker{}

	ids, err := checker.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "")
	if err == nil {
		t.Fatal("UT-KESSEL-LIST-005: expected error for empty username, got nil")
	}
	if !strings.Contains(err.Error(), "username") {
		t.Errorf("UT-KESSEL-LIST-005: error %q should mention 'username'", err.Error())
	}
	if len(ids) != 0 {
		t.Errorf("UT-KESSEL-LIST-005: expected empty slice on error, got %v", ids)
	}
}

// --- KesselClient.ListAuthorizedResources gRPC-level unit tests (F-6, F-7) ---.

// UT-KESSEL-LIST-GRPC-001: KesselClient.ListAuthorizedResources returns IDs from stream.
func TestKesselClientListAuthorizedResourcesReturnsIDs(t *testing.T) {
	checkMock := &mockCheckService{}
	lookupMock := &mockLookupClient{
		responses: []*v1beta1.LookupResourcesResponse{
			{Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "cost_management", Name: "openshift_cluster"},
				Id:   "cluster-x",
			}},
			{Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "cost_management", Name: "openshift_cluster"},
				Id:   "cluster-y",
			}},
		},
	}
	client := NewKesselClient(checkMock, lookupMock)

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "cluster-x" || ids[1] != "cluster-y" {
		t.Errorf("expected [cluster-x cluster-y], got %v", ids)
	}
}

// UT-KESSEL-LIST-GRPC-002: empty stream returns empty slice.
func TestKesselClientListAuthorizedResourcesEmpty(t *testing.T) {
	client := NewKesselClient(&mockCheckService{}, &mockLookupClient{})

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice, got %v", ids)
	}
}

// UT-KESSEL-LIST-GRPC-003: LookupResources gRPC call error is propagated.
func TestKesselClientListAuthorizedResourcesCallError(t *testing.T) {
	lookupMock := &mockLookupClient{
		lookupErr: status.Errorf(codes.Unavailable, "connection refused"),
	}
	client := NewKesselClient(&mockCheckService{}, lookupMock)

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err == nil {
		t.Fatal("expected error for gRPC call failure, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice on error, got %v", ids)
	}
}

// UT-KESSEL-LIST-GRPC-004: stream Recv error is propagated.
func TestKesselClientListAuthorizedResourcesStreamError(t *testing.T) {
	lookupMock := &mockLookupClient{
		streamErr: status.Errorf(codes.Internal, "stream broken"),
	}
	client := NewKesselClient(&mockCheckService{}, lookupMock)

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err == nil {
		t.Fatal("expected error for stream Recv failure, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice on error, got %v", ids)
	}
}

// UT-KESSEL-LIST-GRPC-005: nil lookupClient returns empty slice.
func TestKesselClientListAuthorizedResourcesNilLookup(t *testing.T) {
	client := NewKesselClient(&mockCheckService{})

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "cost_management/openshift_cluster", "read", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice when lookupClient is nil, got %v", ids)
	}
}

// UT-KESSEL-LIST-GRPC-006: invalid resourceType format returns error.
func TestKesselClientListAuthorizedResourcesInvalidResourceType(t *testing.T) {
	client := NewKesselClient(&mockCheckService{}, &mockLookupClient{})

	ids, err := client.ListAuthorizedResources(context.Background(), "org-1", "no-slash-here", "read", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid resourceType format, got nil")
	}
	if !strings.Contains(err.Error(), "namespace/name") {
		t.Errorf("error %q should mention 'namespace/name' format", err.Error())
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice on error, got %v", ids)
	}
}

// mockPermissionChecker implements PermissionChecker for unit tests.
// with both CheckPermission and ListAuthorizedResources using input-sensitive maps.
var _ PermissionChecker = (*mockPermissionChecker)(nil)

type mockPermissionChecker struct {
	allowedTuples map[string]bool
	errorTuples   map[string]error
	authorizedIDs map[string][]string
	listErrors    map[string]error
}

func (m *mockPermissionChecker) CheckPermission(_ context.Context, orgID, permission, username string) (bool, error) {
	key := orgID + "|" + permission + "|" + username
	if err, ok := m.errorTuples[key]; ok {
		return false, err
	}
	return m.allowedTuples[key], nil
}

func (m *mockPermissionChecker) ListAuthorizedResources(_ context.Context, orgID, resourceType, permission, username string) ([]string, error) {
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	key := orgID + "|" + resourceType + "|" + permission + "|" + username
	if err, ok := m.listErrors[key]; ok {
		return nil, err
	}
	if ids, ok := m.authorizedIDs[key]; ok {
		return ids, nil
	}
	return []string{}, nil
}
