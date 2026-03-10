package kessel

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	grpcMaxRetries  = 3
	grpcBackoffBase = 100 * time.Millisecond
)

// RetryBackoff controls the base delay between retry attempts.
// Tests can set this to near-zero to avoid slow runs.
var RetryBackoff = grpcBackoffBase

func pow5(n int) time.Duration {
	r := time.Duration(1)
	for range n {
		r *= 5
	}
	return r
}

func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}

func retryGRPC[T any](fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T
	for attempt := range grpcMaxRetries {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		if !isRetryable(err) {
			return zero, err
		}
		lastErr = err
		if attempt < grpcMaxRetries-1 {
			time.Sleep(RetryBackoff * pow5(attempt))
		}
	}
	return zero, lastErr
}

// PermissionChecker abstracts Kessel permission checks so both the real gRPC.
// client and test mocks implement the same contract.
type PermissionChecker interface {
	CheckPermission(ctx context.Context, orgID, permission, username string) (bool, error)
	ListAuthorizedResources(ctx context.Context, orgID, resourceType, permission, username string) ([]string, error)
}

var _ PermissionChecker = (*KesselClient)(nil)

// KesselClient wraps the Kessel Relations API gRPC clients for both.
// permission checks and resource lookups.
type KesselClient struct {
	checkClient  v1beta1.KesselCheckServiceClient
	lookupClient v1beta1.KesselLookupServiceClient
}

// NewKesselClient creates a KesselClient with the Check and Lookup service clients.
// lookupClient may be nil; if so, ListAuthorizedResources always returns empty.
func NewKesselClient(checkClient v1beta1.KesselCheckServiceClient, lookupClient ...v1beta1.KesselLookupServiceClient) *KesselClient {
	c := &KesselClient{checkClient: checkClient}
	if len(lookupClient) > 0 {
		c.lookupClient = lookupClient[0]
	}
	return c
}

// principalID returns the fully-qualified SpiceDB principal identifier.
// On-prem principals are stored with a "redhat/" prefix by kessel-admin.sh.
// and Koku's access_provider.py to match the Kessel convention.
func principalID(username string) string {
	if strings.Contains(username, "/") {
		return username
	}
	return "redhat/" + username
}

func (k *KesselClient) CheckPermission(ctx context.Context, orgID, permission, username string) (bool, error) {
	if orgID == "" {
		return false, fmt.Errorf("org_id is required")
	}
	if username == "" {
		return false, fmt.Errorf("username is required")
	}

	req := &v1beta1.CheckRequest{
		Resource: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "tenant"},
			Id:   orgID,
		},
		Relation: permission,
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   principalID(username),
			},
		},
	}

	resp, err := retryGRPC(func() (*v1beta1.CheckResponse, error) {
		return k.checkClient.Check(ctx, req)
	})
	if err != nil {
		return false, err
	}

	return resp.Allowed == v1beta1.CheckResponse_ALLOWED_TRUE, nil
}

// ListAuthorizedResources returns the resource IDs the user is authorized to access.
// for the given resource type and permission, using the Relations API LookupResources.
// resourceType should be in "namespace/name" format (e.g. "cost_management/openshift_cluster").
//
// orgID is validated but not sent in the LookupResourcesRequest (the Relations API has no.
// org field). Tenant scoping relies on the ZED schema hierarchy (resource -> workspace -> tenant).
// The middleware calls this once per request, where the identity carries a single orgID.
func (k *KesselClient) ListAuthorizedResources(ctx context.Context, orgID, resourceType, permission, username string) ([]string, error) {
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	if k.lookupClient == nil {
		return []string{}, nil
	}

	parts := strings.SplitN(resourceType, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("resourceType must be in namespace/name format, got %q", resourceType)
	}

	req := &v1beta1.LookupResourcesRequest{
		ResourceType: &v1beta1.ObjectType{Namespace: parts[0], Name: parts[1]},
		Relation:     permission,
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   principalID(username),
			},
		},
	}

	stream, err := retryGRPC(func() (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
		return k.lookupClient.LookupResources(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("LookupResources: %w", err)
	}

	seen := make(map[string]struct{})
	var ids []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("LookupResources stream: %w", err)
		}
		if resp.Resource != nil {
			if _, dup := seen[resp.Resource.Id]; !dup {
				seen[resp.Resource.Id] = struct{}{}
				ids = append(ids, resp.Resource.Id)
			}
		}
	}
	return ids, nil
}
