# Kessel Development Guide

How to set up and develop ros-ocp-backend with Kessel authorization locally.
Kessel is the authorization backend for on-prem deployments, replacing the
SaaS RBAC service which is not available outside of cloud.redhat.com.

## Prerequisites

- Go 1.25+
- Podman and `podman compose` (or Docker Compose)
- [`grpcurl`](https://github.com/fullstorydev/grpcurl#installation) (optional -- only needed for manual verification against the local stack)

---

## 1. Start the Kessel Stack

```bash
podman compose -f docker-compose.kessel.yml up -d
```

This starts:

| Service | Port | Purpose |
|---|---|---|
| PostgreSQL | 5433 | Shared database for SpiceDB and Inventory |
| SpiceDB | 50052 | Authorization engine |
| Relations API | 9000 (gRPC), 8100 (HTTP) | Kessel API -- the endpoint ros-ocp-backend connects to |
| Inventory API | 9081 (gRPC), 8081 (HTTP) | Resource inventory (used by Koku, not ROS) |

The ZED schema is loaded automatically from `dev/schema/schema.zed` via the
`SPICEDB_SCHEMA_FILE` environment variable on the Relations API container.

Wait for services to be healthy:

```bash
podman compose -f docker-compose.kessel.yml ps
```

---

## 2. Run ros-ocp-backend Against Kessel

Set the environment variables to point ros-ocp-backend at the local Kessel stack:

```bash
export AUTHORIZATION_BACKEND=kessel
export KESSEL_RELATIONS_URL=localhost:9000
export RBAC_ENABLE=true
```

Then start the server as you normally would (see
[Dev environment setup](https://github.com/RedHatInsights/ros-ocp-backend/wiki/Dev-environment-setup-(local))).

With these variables, `SelectAuthMiddleware` returns the `KesselMiddleware` instead of
the RBAC middleware. Every request goes through `LookupResources` + `Check` against the
local Relations API on port 9000.

**TLS behavior:** When `KESSEL_RELATIONS_CA_PATH` is empty (the default), the server
connects using plaintext gRPC (matching the docker-compose stack). When a CA path is set,
TLS is enabled with that CA. In production, set `KESSEL_RELATIONS_CA_PATH` to the
appropriate certificate.

> **Note:** On-prem deployments always use Kessel. There is no "switch back to RBAC"
> because the SaaS RBAC service is not available outside of cloud.redhat.com.
> `AUTHORIZATION_BACKEND=kessel` with `RBAC_ENABLE=true` is the expected on-prem
> configuration.

---

## 3. Run Tests

All tests seed their own fixtures programmatically via
[`kessel_seeder.go`](../../internal/testutil/kessel_seeder.go) -- no manual provisioning
is needed.

### Unit tests (no stack needed)

```bash
go test ./internal/kessel/ ./internal/api/middleware/ ./internal/config/ ./internal/utils/sources/ -count=1
```

### Integration tests (stack must be running)

```bash
go test -tags integration ./internal/api/middleware/ -count=1 -v
```

### Contract tests (stack must be running)

```bash
go test -tags contract ./internal/kessel/ -count=1 -v
```

---

## 4. Modify the ZED Schema

The local schema lives at `dev/schema/schema.zed`. The **source of truth** for the
production schema is [`RedHatInsights/rbac-config`](https://github.com/RedHatInsights/rbac-config).

To apply schema changes:

1. Edit `dev/schema/schema.zed`
2. Restart the Relations API to reload:

```bash
podman compose -f docker-compose.kessel.yml restart relations-api
```

3. Run contract tests to validate the schema resolves correctly:

```bash
go test -tags contract ./internal/kessel/ -count=1 -v
```

---

## 5. Tear Down

```bash
podman compose -f docker-compose.kessel.yml down -v
```

The `-v` flag removes volumes (database data). Omit it to preserve data between runs.

---

## 6. Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `connection refused` on port 9000 | Relations API not ready | Wait: `podman compose -f docker-compose.kessel.yml ps` |
| `Check` returns `ALLOWED_FALSE` | Missing tuples in the chain | Verify the full chain exists: role has `t_*_read` → principal, role_binding has `t_granted` → role and `t_subject` → principal, workspace has `t_binding` → role_binding, tenant has `t_default_binding` → role_binding |
| `LookupResources` returns empty | Resource not linked to workspace | Verify `t_workspace` tuple exists on the resource |
| IT/CT fail with `failed to connect` | Stack not running | Start: `podman compose -f docker-compose.kessel.yml up -d` |
| Schema changes not taking effect | Relations API caches schema | Restart: `podman compose -f docker-compose.kessel.yml restart relations-api` |
| All requests return 403 | Missing or wrong `AUTHORIZATION_BACKEND` | Set `AUTHORIZATION_BACKEND=kessel` and `RBAC_ENABLE=true` |
| TLS handshake error on startup | `KESSEL_RELATIONS_CA_PATH` set but stack uses plaintext | Unset `KESSEL_RELATIONS_CA_PATH` for local dev |

---

## References

- Design doc: [kessel-integration.md](./kessel-integration.md)
- Test plan: [kessel-rebac-test-plan.md](../kessel-rebac-test-plan.md)
- ZED schema: [dev/schema/schema.zed](../../dev/schema/schema.zed)
- Test seeder: [internal/testutil/kessel_seeder.go](../../internal/testutil/kessel_seeder.go)
- Production ZED schema: [RedHatInsights/rbac-config](https://github.com/RedHatInsights/rbac-config)
- Dev environment setup: [GitHub Wiki](https://github.com/RedHatInsights/ros-ocp-backend/wiki/Dev-environment-setup-(local))

---

## Appendix: Manual Seeding and Verification (optional)

If you're running the server locally (not tests) and want to make real HTTP requests,
you need tuples in the stack. Tests handle this automatically, but for ad-hoc
development you can seed manually with `grpcurl`.

### Seed a user with cluster read access

```bash
# 1. Create a role with cluster_read permission
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {"resource": {"type": {"namespace": "rbac", "name": "role"}, "id": "cost-admin"},
     "relation": "t_cost_management_openshift_cluster_read",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "user-1"}}}
  ]
}' localhost:9000 kessel.relations.v1beta1.KesselTupleService/CreateTuples

# 2. Create a role_binding granting the role
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {"resource": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "rb-1"},
     "relation": "t_granted",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "role"}, "id": "cost-admin"}}},
    {"resource": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "rb-1"},
     "relation": "t_subject",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "user-1"}}}
  ]
}' localhost:9000 kessel.relations.v1beta1.KesselTupleService/CreateTuples

# 3. Create a workspace under the tenant with the binding
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {"resource": {"type": {"namespace": "rbac", "name": "workspace"}, "id": "ws-org1"},
     "relation": "t_parent",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "tenant"}, "id": "org-1"}}},
    {"resource": {"type": {"namespace": "rbac", "name": "workspace"}, "id": "ws-org1"},
     "relation": "t_binding",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "rb-1"}}}
  ]
}' localhost:9000 kessel.relations.v1beta1.KesselTupleService/CreateTuples

# 4. Bind the tenant's default role_binding
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {"resource": {"type": {"namespace": "rbac", "name": "tenant"}, "id": "org-1"},
     "relation": "t_default_binding",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "rb-1"}}}
  ]
}' localhost:9000 kessel.relations.v1beta1.KesselTupleService/CreateTuples

# 5. Register a cluster resource in the workspace
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {"resource": {"type": {"namespace": "cost_management", "name": "openshift_cluster"}, "id": "cluster-abc-123"},
     "relation": "t_workspace",
     "subject": {"subject": {"type": {"namespace": "rbac", "name": "workspace"}, "id": "ws-org1"}}}
  ]
}' localhost:9000 kessel.relations.v1beta1.KesselTupleService/CreateTuples
```

### Verify

```bash
# Check permission (tenant-level)
grpcurl -plaintext -d '{
  "resource": {"type": {"namespace": "rbac", "name": "tenant"}, "id": "org-1"},
  "relation": "cost_management_openshift_cluster_read",
  "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "user-1"}}
}' localhost:9000 kessel.relations.v1beta1.KesselCheckService/Check

# List authorized resources
grpcurl -plaintext -d '{
  "resource_type": {"namespace": "cost_management", "name": "openshift_cluster"},
  "relation": "read",
  "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "user-1"}}
}' localhost:9000 kessel.relations.v1beta1.KesselLookupService/LookupResources
```
