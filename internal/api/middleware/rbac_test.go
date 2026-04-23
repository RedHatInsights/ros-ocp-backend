package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/ros-ocp-backend/internal/types"
)

func TestAggregatePermissions(t *testing.T) {
	tests := []struct {
		name string
		acls []types.RbacData
		want map[string][]string
		desc string
	}{
		{
			name: "empty acl list",
			acls: []types.RbacData{},
			want: map[string][]string{},
		},
		{
			name: "permission without colon does not panic",
			acls: []types.RbacData{
				{Permission: "no-colon-here"},
			},
			want: map[string][]string{},
		},
		{
			name: "empty permission string does not panic",
			acls: []types.RbacData{
				{Permission: ""},
			},
			want: map[string][]string{},
		},
		{
			name: "wildcard resource type",
			acls: []types.RbacData{
				{Permission: "cost-management:*:read"},
			},
			want: map[string][]string{"*": {}},
		},
		{
			name: "openshift.cluster with no resource definitions",
			acls: []types.RbacData{
				{Permission: "cost-management:openshift.cluster:read"},
			},
			want: map[string][]string{"openshift.cluster": {"*"}},
		},
		{
			name: "openshift.project with string resource definition",
			acls: []types.RbacData{
				{
					Permission: "cost-management:openshift.project:read",
					ResourceDefinitions: []types.RbacResourceDefinitions{
						{AttributeFilter: types.AttributeFilter{Value: "my-project"}},
					},
				},
			},
			want: map[string][]string{"openshift.project": {"my-project"}},
		},
		{
			name: "openshift.node with array resource definition",
			acls: []types.RbacData{
				{
					Permission: "cost-management:openshift.node:read",
					ResourceDefinitions: []types.RbacResourceDefinitions{
						{AttributeFilter: types.AttributeFilter{Value: []interface{}{"node-a", "node-b"}}},
					},
				},
			},
			want: map[string][]string{"openshift.node": {"node-a", "node-b"}},
		},
		{
			name: "non-openshift resource type is ignored",
			acls: []types.RbacData{
				{Permission: "cost-management:aws.account:read"},
			},
			want: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregate_permissions(tt.acls)
			if len(got) != len(tt.want) {
				t.Errorf("aggregate_permissions() returned %d keys, want %d.\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for k, wantVals := range tt.want {
				gotVals, ok := got[k]
				if !ok {
					t.Errorf("missing key %q in result", k)
					continue
				}
				if len(gotVals) != len(wantVals) {
					t.Errorf("key %q: got %v, want %v", k, gotVals, wantVals)
				}
			}
		})
	}
}

func TestRequestUserAccess_Non2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	acls := request_user_access(srv.URL, "dummyIdentity")
	if len(acls) != 0 {
		t.Errorf("expected empty acls on 500 response, got %d", len(acls))
	}
}

func TestRequestUserAccess_ValidResponse(t *testing.T) {
	rbacResp := types.RbacResponse{
		Data: []types.RbacData{
			{Permission: "cost-management:openshift.cluster:read"},
		},
	}
	body, _ := json.Marshal(rbacResp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	acls := request_user_access(srv.URL, "dummyIdentity")
	if len(acls) != 1 {
		t.Errorf("expected 1 acl, got %d", len(acls))
	}
}

func TestRequestUserAccess_ConnectionRefused(t *testing.T) {
	// Calling an unreachable URL should return empty, not panic
	acls := request_user_access("http://127.0.0.1:1/unreachable", "dummyIdentity")
	if len(acls) != 0 {
		t.Errorf("expected empty acls on connection error, got %d", len(acls))
	}
}

func TestRequestUserAccess_GarbageJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not json"))
	}))
	defer srv.Close()

	acls := request_user_access(srv.URL, "dummyIdentity")
	if len(acls) != 0 {
		t.Errorf("expected empty acls on garbage JSON, got %d", len(acls))
	}
}
