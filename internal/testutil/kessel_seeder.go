//go:build contract || integration

package testutil

import (
	"context"
	"fmt"
	"time"

	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// KesselSeeder seeds and cleans up tuples via the Kessel Relations API.
// It never talks to SpiceDB directly.
type KesselSeeder struct {
	conn        *grpc.ClientConn
	tupleClient v1beta1.KesselTupleServiceClient
	checkClient v1beta1.KesselCheckServiceClient
}

// NewKesselSeeder connects to the Kessel Relations API at the given endpoint.
func NewKesselSeeder(endpoint string) (*KesselSeeder, error) {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connecting to Kessel Relations API at %s: %w", endpoint, err)
	}
	return &KesselSeeder{
		conn:        conn,
		tupleClient: v1beta1.NewKesselTupleServiceClient(conn),
		checkClient: v1beta1.NewKesselCheckServiceClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (s *KesselSeeder) Close() error {
	return s.conn.Close()
}

// CheckClient returns the KesselCheckServiceClient for use in tests.
func (s *KesselSeeder) CheckClient() v1beta1.KesselCheckServiceClient {
	return s.checkClient
}

// SeedTuples creates tuples via CreateTuples with Upsert=true so re-seeding is idempotent.
func (s *KesselSeeder) SeedTuples(ctx context.Context, rels []*v1beta1.Relationship) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.tupleClient.CreateTuples(ctx, &v1beta1.CreateTuplesRequest{
		Upsert: true,
		Tuples: rels,
	})
	if err != nil {
		return fmt.Errorf("CreateTuples: %w", err)
	}
	return nil
}

// DeleteTuples deletes tuples matching each relationship's resource+relation+subject.
func (s *KesselSeeder) DeleteTuples(ctx context.Context, rels []*v1beta1.Relationship) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, rel := range rels {
		filter := &v1beta1.RelationTupleFilter{}

		if rel.Resource != nil && rel.Resource.Type != nil {
			ns := rel.Resource.Type.Namespace
			name := rel.Resource.Type.Name
			filter.ResourceNamespace = &ns
			filter.ResourceType = &name
		}
		if rel.Resource != nil {
			id := rel.Resource.Id
			filter.ResourceId = &id
		}
		relation := rel.Relation
		filter.Relation = &relation

		_, err := s.tupleClient.DeleteTuples(ctx, &v1beta1.DeleteTuplesRequest{
			Filter: filter,
		})
		if err != nil {
			return fmt.Errorf("DeleteTuples for %s/%s:%s#%s: %w",
				rel.Resource.Type.Namespace, rel.Resource.Type.Name, rel.Resource.Id, rel.Relation, err)
		}
	}
	return nil
}

// Rel builds a v1beta1.Relationship for seeding.
func Rel(resNs, resName, resID, relation, subNs, subName, subID string) *v1beta1.Relationship {
	return &v1beta1.Relationship{
		Resource: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{Namespace: resNs, Name: resName},
			Id:   resID,
		},
		Relation: relation,
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: subNs, Name: subName},
				Id:   subID,
			},
		},
	}
}

// RelWithSubRelation builds a Relationship where the subject has a relation (e.g. group#member).
func RelWithSubRelation(resNs, resName, resID, relation, subNs, subName, subID, subRelation string) *v1beta1.Relationship {
	r := Rel(resNs, resName, resID, relation, subNs, subName, subID)
	r.Subject.Relation = &subRelation
	return r
}

// Check calls the Kessel Check endpoint and returns whether the permission is allowed.
func (s *KesselSeeder) Check(ctx context.Context, orgID, permission, username string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := s.checkClient.Check(ctx, &v1beta1.CheckRequest{
		Resource: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "tenant"},
			Id:   orgID,
		},
		Relation: permission,
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   username,
			},
		},
	})
	if err != nil {
		return false, err
	}
	return resp.Allowed == v1beta1.CheckResponse_ALLOWED_TRUE, nil
}

// WaitForConsistency polls a Check until it returns the expected result or times out.
// Useful because SpiceDB revision quantization can delay visibility.
func (s *KesselSeeder) WaitForConsistency(ctx context.Context, orgID, permission, username string, wantAllowed bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		allowed, err := s.Check(ctx, orgID, permission, username)
		if err == nil && allowed == wantAllowed {
			return nil
		}
		time.Sleep(interval)
		if interval < 5*time.Second {
			interval *= 2
		}
	}
	return fmt.Errorf("consistency timeout (%.0fs): Check(%s, %s, %s) did not return %v",
		timeout.Seconds(), orgID, permission, username, wantAllowed)
}
