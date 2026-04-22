# Kessel/ReBAC Integration -- Test Plan (ros-ocp-backend)

| Field         | Value                                                                        |
|---------------|------------------------------------------------------------------------------|
| Jira          | [FLPATH-3338](https://issues.redhat.com/browse/FLPATH-3338)                 |
| Parent story  | [FLPATH-2690](https://issues.redhat.com/browse/FLPATH-2690)                 |
| DD Reference  | [ros-ocp-backend ReBAC integration plan](../../.cursor/plans/ros-ocp-backend_rebac_integration_99cb4ca1.plan.md) |
| Koku DD       | [kessel-ocp-detailed-design.md](../../../koku/docs/architecture/kessel-integration/kessel-ocp-detailed-design.md) |
| Author        | Jordi Gil                                                                    |
| Status        | Draft                                                                        |
| Created       | 2026-02-13                                                                  |
| Last updated  | 2026-02-13                                                                  |

## Table of Contents

1. [Conventions](#1-conventions)
2. [Tier 1 -- Unit Tests (UT)](#2-tier-1----unit-tests-ut)
3. [Tier 2 -- Integration Tests (IT)](#3-tier-2----integration-tests-it)
4. [Tier 3 -- Contract Tests (CT)](#4-tier-3----contract-tests-ct)
5. [Coverage Summary](#5-coverage-summary)

---

## 1. Conventions

### 1.1 Scenario ID Format

`{TIER}-{MODULE}-{FEATURE}-{NNN}`

| Segment | Values | Description |
|---------|--------|-------------|
| TIER | `UT`, `IT`, `CT` | Unit, Integration, Contract |
| MODULE | `CFG`, `KESSEL`, `MW`, `RBAC`, `SRC` | ros-ocp-backend module where the test code lives |
| FEATURE | Short mnemonic | Feature under test |
| NNN | `001`-`999` | Sequential scenario number |

### 1.2 Module Reference

| Code | Go package | Covers |
|------|------------|--------|
| `CFG` | `internal/config/` | Config struct, env var parsing, defaults |
| `KESSEL` | `internal/kessel/` | Kessel gRPC client, CheckPermission wrapper |
| `MW` | `internal/api/middleware/` | Kessel middleware, server middleware selection |
| `RBAC` | `internal/api/middleware/` | Existing RBAC middleware, aggregate_permissions |
| `SRC` | `internal/utils/sources/` | GetCostApplicationID, env var fallback |

### 1.3 Feature Mnemonics

| Mnemonic | Feature |
|----------|---------|
| `BACKEND` | Authorization backend selection (RBAC vs Kessel) |
| `CHECK` | CheckPermission gRPC call |
| `AUTH` | Kessel middleware authorization flow |
| `PERM` | RBAC permission aggregation |
| `ACCESS` | RBAC middleware HTTP API flow |
| `APPID` | Cost application type ID resolution |
| `REL` | Relations API contract (CheckPermission) |
| `SCHEMA` | ZED schema permission resolution |
| `GRP` | Group membership inheritance |
| `LIST` | ListAuthorizedResources / LookupResources |
| `SLO` | LookupResources + Check fallback |
| `INV` | Kessel Inventory config |
| `SEED` | Kessel Relations API CreateTuples seeding |

### 1.4 Priority Levels

| Level | Meaning | Triage guidance |
|-------|---------|-----------------|
| P0 (Critical) | Core authorization contract, blocks all downstream | Must fix immediately; blocks PR merge |
| P1 (High) | Key feature, significant user impact | Fix before phase checkpoint |
| P2 (Medium) | Important but not blocking | Fix before final PR merge |
| P3 (Low) | Defensive, unlikely paths | Can defer to follow-up |

### 1.5 Per-Scenario Format

Each scenario follows an IEEE 829-inspired structure with Priority, Business Value, Fixtures, BDD Steps, and Acceptance Criteria.

### 1.6 Tier Infrastructure

| Tier | Runner | CI? | Kessel backend |
|------|--------|-----|----------------|
| UT | `go test` / Ginkgo | Yes | Input-sensitive mock (see 1.8) |
| IT | `go test -tags integration` / Ginkgo | Local | Real SpiceDB + Relations API via Podman Compose |
| CT | `go test -tags contract` | Local | Real SpiceDB + Relations API via Podman Compose |

IT and CT share the same Podman Compose infrastructure (PostgreSQL + SpiceDB + Kessel Relations API + Inventory API). Schema is loaded at Relations API startup via `SPICEDB_SCHEMA_FILE` volume mount (no `WriteSchema` in test code). Test data is seeded via Kessel Relations API `CreateTuples`. The difference is scope: IT tests the full application middleware pipeline (identity + Kessel middleware + stub handler), while CT tests the gRPC contract directly (seed via `CreateTuples`, check via Kessel Relations API `Check` and `LookupResources`) with no application middleware code involved.

### 1.7 Scope: OpenShift-Only Permissions

ros-ocp-backend serves only OpenShift resource optimization recommendations. Unlike Koku (which checks 10 resource types including AWS, Azure, GCP, cost_model, and settings), ros-ocp-backend checks exactly 3 Kessel permissions:

| RBAC resource type | Kessel permission name | ZED relation |
|--------------------|------------------------|--------------|
| `openshift.cluster` | `cost_management_openshift_cluster_read` | `t_cost_management_openshift_cluster_read` |
| `openshift.node` | `cost_management_openshift_node_read` | `t_cost_management_openshift_node_read` |
| `openshift.project` | `cost_management_openshift_project_read` | `t_cost_management_openshift_project_read` |

**Note on `openshift.node`:** The `AddRBACFilter` function in `internal/rbac/query_builder.go` (called from model layer) only uses `openshift.cluster` and `openshift.project` for DB query scoping. `openshift.node` is an authorization-only permission: it controls whether the user is allowed to make API requests (middleware gate), but has no effect on which data rows are returned. This matches the existing RBAC behavior -- the RBAC middleware fetches all three permission types, but `add_rbac_filter` only applies cluster and project filters. This is an existing limitation, not introduced by this change.

### 1.8 Mock Design: Input-Sensitive Mocks

All UT and middleware tests use **input-sensitive mocks** rather than mock-interaction assertions. The mock returns results only when the exact input tuple matches a configured entry. This validates correct parameter propagation through observable outcomes (returned bool, HTTP status code, permissions map) without asserting on mock call arguments.

```go
type inputSensitiveMock struct {
    allowedTuples map[string]bool     // key = "orgID|permission|username"
    errorTuples   map[string]error    // key = "orgID|permission|username"
    authorizedIDs map[string][]string // key = "orgID|resourceType|permission|username"
    listErrors    map[string]error    // key = "orgID|resourceType|permission|username"
}
func (m *inputSensitiveMock) CheckPermission(ctx, orgID, perm, user) (bool, error) {
    key := orgID + "|" + perm + "|" + user
    if err, ok := m.errorTuples[key]; ok { return false, err }
    return m.allowedTuples[key], nil
}
func (m *inputSensitiveMock) ListAuthorizedResources(ctx, orgID, resType, perm, user) ([]string, error) {
    key := orgID + "|" + resType + "|" + perm + "|" + user
    if err, ok := m.listErrors[key]; ok { return nil, err }
    if ids, ok := m.authorizedIDs[key]; ok { return ids, nil }
    return []string{}, nil
}
```

### 1.9 Design Decision: HTTP 403 on Kessel Outage

When Kessel is unreachable, ros-ocp-backend returns HTTP 403 (Forbidden), not HTTP 424 (Failed Dependency) as Koku does. This is intentional: ros-ocp-backend's existing RBAC middleware already returns 403 when the RBAC API is unavailable. Maintaining behavioral consistency within ros-ocp-backend across both authorization backends is more important than cross-project consistency with Koku.

The Kessel middleware uses `echo.NewHTTPError(http.StatusForbidden, "User is not authorized")` for 403 responses, matching the exact error format and message of the existing RBAC middleware (`rbac.go:27`). For 401 responses (missing/invalid identity), it uses `echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")`.

### 1.10 Assertion Quality Rules

Every assertion validates a specific business outcome, not mere existence:

- **Forbidden**: `Expect(x).ToNot(BeNil())`, `Expect(x).ToNot(BeEmpty())`, `Expect(x).To(BeTrue())` used alone
- **Required**: `Expect(permissions).To(HaveLen(3))`, `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`, `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Error assertions use `Expect(err).ToNot(HaveOccurred())` or `Expect(err).To(MatchError(ContainSubstring("...")))` -- never `Expect(err).To(BeNil())`

### 1.11 Design Decision: `RBACEnabled` as Authorization Toggle

`RBACEnabled` (`RBAC_ENABLE` env var) serves as the master on/off switch for authorization. `AuthorizationBackend` (`AUTHORIZATION_BACKEND` env var, default `"rbac"`) selects which system implements it. The three valid deployment states are:

| State | `RBAC_ENABLE` | `AUTHORIZATION_BACKEND` | Middleware | DB filter active? |
|-------|--------------|------------------------|------------|-------------------|
| No auth (local dev) | `false` | `rbac` (default) | None | No |
| RBAC (SaaS / Clowder) | `true` | `rbac` (default) | RBAC | Yes |
| Kessel (on-prem) | `true` | `kessel` | Kessel | Yes (no-op, see below) |

`AddRBACFilter` in `internal/rbac/query_builder.go` (called from model layer) is gated on `cfg.RBACEnabled`. This guard is unchanged. For the Kessel path, the middleware now uses `LookupResources` (via `ListAuthorizedResources`) for `openshift.cluster` and `openshift.project`, producing either specific resource IDs or `["*"]` (via `CheckPermission` fallback). `AddRBACFilter` already handles both scoped values and wildcards correctly.

The middleware selection logic in `server.go` uses `SelectAuthMiddleware(cfg, kesselClient)`:

```go
if authMW := SelectAuthMiddleware(cfg, kesselClient); authMW != nil {
    v1.Use(authMW)
}
```

`SelectAuthMiddleware` returns nil when `RBACEnabled` is false, `KesselMiddleware` when `AuthorizationBackend` is `"kessel"` (case-insensitive), and `Rbac` otherwise.

### 1.12 Implementation Note: Config Singleton Reset

`internal/config/config.go` uses a package-level singleton (`var cfg *Config = nil`) with lazy initialization in `GetConfig()`. Once initialized, subsequent calls return the cached value regardless of env var changes. UT-CFG-BACKEND-* scenarios set env vars and expect the config to reflect them. The implementation must either:
- Export a `ResetConfig()` test helper that sets `cfg = nil`, called in `BeforeEach` / `t.Cleanup`
- Or test the internal `initConfig()` directly, bypassing the singleton

The existing `config_test.go` tests `getEnvWithDefault()` (a pure function) which avoids this issue. The new Kessel config tests will need the reset approach.

### 1.13 Context Key Contract

Both the RBAC middleware and the Kessel middleware MUST set permissions in the Echo context under the key `"user.permissions"` with type `map[string][]string`. The handler code (`get_user_permissions()` in `internal/api/utils.go`) reads from this exact key. If the key is absent or the type does not match, `get_user_permissions()` returns an empty map, and `add_rbac_filter` applies no scoping (which, when `RBACEnabled=true`, means no wildcard match is found and the query runs unscoped -- a permissive fallback).

---

## 2. Tier 1 -- Unit Tests (UT)

Coverage target: >80% on all new/modified code.

---

### UT-CFG-BACKEND-001: Default authorization backend is RBAC

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Operators upgrading from a pre-Kessel deployment must continue using RBAC without any configuration change |

**Fixtures:**
- No `AUTHORIZATION_BACKEND` env var set
- Fresh `Config` initialization (non-Clowder path)

**Steps:**
- **Given** the `AUTHORIZATION_BACKEND` environment variable is not set
- **When** the configuration is initialized
- **Then** `cfg.AuthorizationBackend` equals `"rbac"`

**Acceptance Criteria:**
- `Expect(cfg.AuthorizationBackend).To(Equal("rbac"))` -- backward compatibility preserved

---

### UT-CFG-BACKEND-002: Operator can select Kessel backend via environment variable

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | On-prem operators must be able to enable Kessel authorization by setting a single environment variable |

**Fixtures:**
- `AUTHORIZATION_BACKEND=kessel` env var set
- Fresh `Config` initialization

**Steps:**
- **Given** `AUTHORIZATION_BACKEND` is set to `"kessel"`
- **When** the configuration is initialized
- **Then** `cfg.AuthorizationBackend` equals `"kessel"`

**Acceptance Criteria:**
- `Expect(cfg.AuthorizationBackend).To(Equal("kessel"))` -- Kessel is selectable

---

### UT-CFG-BACKEND-003: Default Kessel Relations URL for local development

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | Developers running locally must get a sensible default without configuring the Relations API URL |

**Fixtures:**
- No `KESSEL_RELATIONS_URL` env var set

**Steps:**
- **Given** `KESSEL_RELATIONS_URL` is not set
- **When** the configuration is initialized
- **Then** `cfg.KesselRelationsURL` equals `"localhost:9000"`

**Acceptance Criteria:**
- `Expect(cfg.KesselRelationsURL).To(Equal("localhost:9000"))`

---

### UT-CFG-BACKEND-004: Operator can override Kessel Relations URL

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Operators must be able to point ros-ocp-backend at a non-default Kessel endpoint |

**Fixtures:**
- `KESSEL_RELATIONS_URL=kessel:8443` env var set

**Steps:**
- **Given** `KESSEL_RELATIONS_URL` is set to `"kessel:8443"`
- **When** the configuration is initialized
- **Then** `cfg.KesselRelationsURL` equals `"kessel:8443"`

**Acceptance Criteria:**
- `Expect(cfg.KesselRelationsURL).To(Equal("kessel:8443"))`

---

### UT-CFG-BACKEND-005: CA path defaults to empty (system trust store)

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | When no custom CA is configured, the gRPC client uses the system trust store, which works for production deployments with properly signed certificates |

**Fixtures:**
- No `KESSEL_RELATIONS_CA_PATH` env var set

**Steps:**
- **Given** `KESSEL_RELATIONS_CA_PATH` is not set
- **When** the configuration is initialized
- **Then** `cfg.KesselRelationsCAPath` equals `""`

**Acceptance Criteria:**
- `Expect(cfg.KesselRelationsCAPath).To(Equal(""))`

---

### UT-CFG-BACKEND-006: Operator can set custom CA path for self-signed certificates

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | On-prem deployments using self-signed or internal PKI certificates must be able to provide a custom CA so the gRPC client trusts the Kessel server |

**Fixtures:**
- `KESSEL_RELATIONS_CA_PATH=/etc/pki/tls/certs/kessel-ca.pem` env var set

**Steps:**
- **Given** `KESSEL_RELATIONS_CA_PATH` is set to `"/etc/pki/tls/certs/kessel-ca.pem"`
- **When** the configuration is initialized
- **Then** `cfg.KesselRelationsCAPath` equals `"/etc/pki/tls/certs/kessel-ca.pem"`

**Acceptance Criteria:**
- `Expect(cfg.KesselRelationsCAPath).To(Equal("/etc/pki/tls/certs/kessel-ca.pem"))`

---

### UT-CFG-BACKEND-007: Preshared key is empty by default

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | Default empty key prevents accidental auth bypass; operators must explicitly set it |

**Fixtures:**
- No `SPICEDB_PRESHARED_KEY` env var set

**Steps:**
- **Given** `SPICEDB_PRESHARED_KEY` is not set
- **When** the configuration is initialized
- **Then** `cfg.SpiceDBPresharedKey` equals `""`

**Acceptance Criteria:**
- `Expect(cfg.SpiceDBPresharedKey).To(Equal(""))`

---

### UT-CFG-BACKEND-008: Operator can set SpiceDB preshared key

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | The preshared key authenticates ros-ocp-backend to SpiceDB; without it, gRPC calls are rejected |

**Fixtures:**
- `SPICEDB_PRESHARED_KEY=secret` env var set

**Steps:**
- **Given** `SPICEDB_PRESHARED_KEY` is set to `"secret"`
- **When** the configuration is initialized
- **Then** `cfg.SpiceDBPresharedKey` equals `"secret"`

**Acceptance Criteria:**
- `Expect(cfg.SpiceDBPresharedKey).To(Equal("secret"))`

---

### UT-CFG-BACKEND-009: Existing RBAC config defaults are preserved

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Adding Kessel config must not break existing RBAC-based deployments; when no RBAC env vars are set, `RBACEnabled` defaults to false (local dev / no auth) |

**Fixtures:**
- No RBAC env vars set (non-Clowder path)
- No `AUTHORIZATION_BACKEND` env var set

**Steps:**
- **Given** no RBAC or Kessel environment variables are set
- **When** the configuration is initialized
- **Then** `cfg.RBACEnabled` equals `false` and `cfg.AuthorizationBackend` equals `"rbac"`

**Acceptance Criteria:**
- `Expect(cfg.RBACEnabled).To(Equal(false))` -- local dev: no auth
- `Expect(cfg.AuthorizationBackend).To(Equal("rbac"))` -- default backend

---

### UT-CFG-BACKEND-010: Kessel on-prem configuration is valid

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | On-prem deployments must be able to enable Kessel authorization with `RBAC_ENABLE=true` (authorization active) and `AUTHORIZATION_BACKEND=kessel` |

**Fixtures:**
- `RBAC_ENABLE=true` env var set
- `AUTHORIZATION_BACKEND=kessel` env var set

**Steps:**
- **Given** `RBAC_ENABLE` is `true` and `AUTHORIZATION_BACKEND` is `kessel`
- **When** the configuration is initialized
- **Then** `cfg.RBACEnabled` equals `true` and `cfg.AuthorizationBackend` equals `"kessel"`

**Acceptance Criteria:**
- `Expect(cfg.RBACEnabled).To(Equal(true))`
- `Expect(cfg.AuthorizationBackend).To(Equal("kessel"))`

---

### UT-CFG-BACKEND-011: RBACEnabled=false disables all authorization regardless of backend

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Setting `RBAC_ENABLE=false` must disable authorization entirely, even if `AUTHORIZATION_BACKEND=kessel` is set; this prevents misconfiguration where a developer sets the backend but forgets that auth is off |

**Fixtures:**
- `RBAC_ENABLE=false` env var set
- `AUTHORIZATION_BACKEND=kessel` env var set

**Steps:**
- **Given** `RBAC_ENABLE` is `false` and `AUTHORIZATION_BACKEND` is `kessel`
- **When** the configuration is initialized
- **Then** `cfg.RBACEnabled` equals `false` and `cfg.AuthorizationBackend` equals `"kessel"`

**Acceptance Criteria:**
- `Expect(cfg.RBACEnabled).To(Equal(false))` -- master switch is off
- `Expect(cfg.AuthorizationBackend).To(Equal("kessel"))` -- backend is set but unused
- When passed to the server middleware registration helper, no middleware is registered (tested in UT-MW-BACKEND-001)

---

### UT-KESSEL-CHECK-001: Authorized user gets access through Kessel

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel confirms a user has a permission, the client must report that access is granted |

**Fixtures:**
- Input-sensitive mock allows tuple `("org-1", "cost_management_openshift_cluster_read", "user-1")`

**Steps:**
- **Given** Kessel has granted `cost_management_openshift_cluster_read` to `user-1` in `org-1`
- **When** `CheckPermission(ctx, "org-1", "cost_management_openshift_cluster_read", "user-1")` is called
- **Then** the client returns `true, nil`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(true))`
- `Expect(err).ToNot(HaveOccurred())`
- The mock only returns ALLOWED for the exact tuple, implicitly validating that the client constructs the correct protobuf request (resource type `rbac/tenant`, resource ID `org-1`, relation `cost_management_openshift_cluster_read`, subject `rbac/principal:user-1`)

---

### UT-KESSEL-CHECK-002: Unauthorized user is denied by Kessel

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel denies a permission, the client must report that access is denied -- never grant access by default |

**Fixtures:**
- Input-sensitive mock allows tuple for `user-1` only (different user)

**Steps:**
- **Given** Kessel has not granted any permissions to `user-2` in `org-1`
- **When** `CheckPermission(ctx, "org-1", "cost_management_openshift_cluster_read", "user-2")` is called
- **Then** the client returns `false, nil`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(false))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-KESSEL-CHECK-003: Wrong organization is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user authorized in one organization must not gain access in another -- tenant isolation is a security invariant |

**Fixtures:**
- Input-sensitive mock allows tuple `("org-1", "cost_management_openshift_cluster_read", "user-1")`

**Steps:**
- **Given** `user-1` is authorized in `org-1`
- **When** `CheckPermission(ctx, "org-99", "cost_management_openshift_cluster_read", "user-1")` is called with a different org
- **Then** the client returns `false, nil`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(false))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-KESSEL-CHECK-004: Kessel unreachable fails closed

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel is down, users must be denied access rather than silently granted -- fail-closed is a security requirement |

**Fixtures:**
- Mock returns `status.Error(codes.Unavailable, "connection refused")` for all inputs

**Steps:**
- **Given** the Kessel Relations API is unreachable
- **When** `CheckPermission` is called
- **Then** the client returns `false` and an error containing `"connection refused"`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(false))`
- `Expect(err).To(MatchError(ContainSubstring("connection refused")))`

---

### UT-KESSEL-CHECK-005: Empty org_id is rejected before gRPC call

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Malformed identity data must be caught early with a clear error, not sent to Kessel as an invalid request |

**Fixtures:**
- No mock configuration (mock should not be called)

**Steps:**
- **Given** the caller provides an empty `org_id`
- **When** `CheckPermission(ctx, "", "cost_management_openshift_cluster_read", "user-1")` is called
- **Then** the client returns `false` and an error containing `"org_id"`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(false))`
- `Expect(err).To(MatchError(ContainSubstring("org_id")))`

---

### UT-KESSEL-CHECK-006: Empty username is rejected before gRPC call

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | A request without a username cannot be authorized; it must fail with a descriptive error |

**Fixtures:**
- No mock configuration (mock should not be called)

**Steps:**
- **Given** the caller provides an empty `username`
- **When** `CheckPermission(ctx, "org-1", "cost_management_openshift_cluster_read", "")` is called
- **Then** the client returns `false` and an error containing `"username"`

**Acceptance Criteria:**
- `Expect(allowed).To(Equal(false))`
- `Expect(err).To(MatchError(ContainSubstring("username")))`

---

### UT-KESSEL-LIST-001: ListAuthorizedResources returns specific resource IDs

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel has per-resource bindings, the client must return the specific IDs the user is authorized to access |

**Fixtures:**
- Mock returns `["cluster-a", "cluster-b"]` for `("org-1", "cost_management/openshift_cluster", "read", "user-1")`

**Steps:**
- **Given** Kessel has resource-level bindings granting access to 2 clusters
- **When** `ListAuthorizedResources(ctx, "org-1", "cost_management/openshift_cluster", "read", "user-1")` is called
- **Then** the client returns `["cluster-a", "cluster-b"], nil`

---

### UT-KESSEL-LIST-002: Empty result when no resource bindings exist

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When no resources are bound, the empty list must be returned so the middleware can fall back to tenant-level Check |

**Fixtures:**
- `mockPermissionChecker` with empty `authorizedIDs` map

**Steps:**
- **Given** no resource bindings exist for `user-no-access`
- **When** `ListAuthorizedResources("org-1", "cost_management/openshift_cluster", "read", "user-no-access")` is called
- **Then** the returned slice is empty and no error is returned

**Acceptance Criteria:**
- `len(ids) == 0`
- `err == nil`

---

### UT-KESSEL-LIST-003: gRPC Unavailable error is propagated

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Network errors during resource listing must be surfaced to the caller for proper fallback handling |

**Fixtures:**
- `mockPermissionChecker` with `listErrors` returning `codes.Unavailable`

**Steps:**
- **Given** the Kessel Relations API is unavailable
- **When** `ListAuthorizedResources` is called
- **Then** a gRPC `Unavailable` error is returned and the ID slice is empty

**Acceptance Criteria:**
- `err != nil` with `status.Code(err) == codes.Unavailable`
- `len(ids) == 0`

---

### UT-KESSEL-LIST-004: Empty orgID is rejected before gRPC call

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Input validation prevents malformed gRPC requests |

**Fixtures:**
- `mockPermissionChecker` (any configuration)

**Steps:**
- **Given** an empty `orgID`
- **When** `ListAuthorizedResources("", ...)` is called
- **Then** an error mentioning `org_id` is returned

**Acceptance Criteria:**
- `err != nil` and `err.Error()` contains `"org_id"`
- `len(ids) == 0`

---

### UT-KESSEL-LIST-005: Empty username is rejected before gRPC call

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Input validation prevents malformed gRPC requests |

**Fixtures:**
- `mockPermissionChecker` (any configuration)

**Steps:**
- **Given** an empty `username`
- **When** `ListAuthorizedResources(..., "")` is called
- **Then** an error mentioning `username` is returned

**Acceptance Criteria:**
- `err != nil` and `err.Error()` contains `"username"`
- `len(ids) == 0`

---

### UT-KESSEL-LIST-GRPC-001: KesselClient.ListAuthorizedResources returns IDs from gRPC stream

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Validates that the real `KesselClient` correctly processes the streaming gRPC response from `LookupResources` and collects resource IDs |

**Fixtures:**
- `mockLookupClient` returning 2 `LookupResourcesResponse` with resource IDs `"cluster-x"`, `"cluster-y"`

**Steps:**
- **Given** a `KesselClient` constructed with `NewKesselClient(checkMock, lookupMock)`
- **When** `ListAuthorizedResources("org-1", "cost_management/openshift_cluster", "read", "user-1")` is called
- **Then** the returned slice is `["cluster-x", "cluster-y"]`

**Acceptance Criteria:**
- `len(ids) == 2` and `ids[0] == "cluster-x"` and `ids[1] == "cluster-y"`

---

### UT-KESSEL-LIST-GRPC-002: Empty gRPC stream returns empty slice

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When no resources match, the stream returns EOF immediately; client must handle gracefully |

**Fixtures:**
- `mockLookupClient` with empty `responses`

**Steps:**
- **Given** a `KesselClient` with an empty lookup stream
- **When** `ListAuthorizedResources` is called
- **Then** an empty slice and nil error are returned

**Acceptance Criteria:**
- `len(ids) == 0` and `err == nil`

---

### UT-KESSEL-LIST-GRPC-003: LookupResources gRPC call error is propagated

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | When the initial `LookupResources` gRPC call fails (e.g. connection refused), the error is surfaced |

**Fixtures:**
- `mockLookupClient` with `lookupErr` set to `codes.Unavailable`

**Steps:**
- **Given** `LookupResources()` returns a gRPC error
- **When** `ListAuthorizedResources` is called on `KesselClient`
- **Then** the error is propagated and the ID slice is nil/empty

**Acceptance Criteria:**
- `err != nil` and `len(ids) == 0`

---

### UT-KESSEL-LIST-GRPC-004: Stream Recv error is propagated

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | When the stream breaks mid-receive, the error is surfaced |

**Fixtures:**
- `mockLookupClient` with `streamErr` set to `codes.Internal`

**Steps:**
- **Given** `stream.Recv()` returns a non-EOF error
- **When** `ListAuthorizedResources` is called on `KesselClient`
- **Then** the error is propagated

**Acceptance Criteria:**
- `err != nil` and `len(ids) == 0`

---

### UT-KESSEL-LIST-GRPC-005: Nil lookupClient returns empty slice

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | When `KesselClient` is constructed without a lookup client, `ListAuthorizedResources` degrades gracefully |

**Fixtures:**
- `KesselClient` created via `NewKesselClient(checkMock)` (no lookup client)

**Steps:**
- **Given** `lookupClient` is nil
- **When** `ListAuthorizedResources` is called
- **Then** an empty slice and nil error are returned

**Acceptance Criteria:**
- `len(ids) == 0` and `err == nil`

---

### UT-KESSEL-LIST-GRPC-006: Invalid resourceType format returns error

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | A `resourceType` not in `namespace/name` format must be rejected before making a gRPC call |

**Fixtures:**
- `KesselClient` with a valid `mockLookupClient`

**Steps:**
- **Given** `resourceType` is `"no-slash-here"` (missing `/`)
- **When** `ListAuthorizedResources` is called on `KesselClient`
- **Then** an error mentioning `"namespace/name"` is returned

**Acceptance Criteria:**
- `err != nil` and `err.Error()` contains `"namespace/name"`
- `len(ids) == 0`

---

### UT-CFG-INV-001: Default Kessel Inventory URL for local development

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | Developers running locally must get a sensible default for the Inventory API |

**Fixtures:**
- No `KESSEL_INVENTORY_URL` env var set (default)

**Steps:**
- **Given** `KESSEL_INVENTORY_URL` is not set
- **When** `GetConfig()` is called
- **Then** `cfg.KesselInventoryURL` equals the default `"localhost:9081"`

**Acceptance Criteria:**
- `cfg.KesselInventoryURL` equals `"localhost:9081"`

---

### UT-CFG-INV-002: Operator can override Kessel Inventory URL

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Operators must be able to point to a non-default Inventory API endpoint |

**Fixtures:**
- `KESSEL_INVENTORY_URL` env var set to `"inventory.example.com:9081"`

**Steps:**
- **Given** `KESSEL_INVENTORY_URL` is set to `"inventory.example.com:9081"`
- **When** `GetConfig()` is called
- **Then** `cfg.KesselInventoryURL` equals `"inventory.example.com:9081"`

**Acceptance Criteria:**
- `cfg.KesselInventoryURL` equals `"inventory.example.com:9081"`

---

### UT-CFG-INV-003: Operator can set custom Inventory CA path

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | On-prem deployments using self-signed certificates need a custom CA path |

**Fixtures:**
- `KESSEL_INVENTORY_CA_PATH` env var set to `"/etc/pki/ca.crt"`

**Steps:**
- **Given** `KESSEL_INVENTORY_CA_PATH` is set to `"/etc/pki/ca.crt"`
- **When** `GetConfig()` is called
- **Then** `cfg.KesselInventoryCAPath` equals `"/etc/pki/ca.crt"`

**Acceptance Criteria:**
- `cfg.KesselInventoryCAPath` equals `"/etc/pki/ca.crt"`

---

### UT-MW-SLO-001: ListAuthorizedResources returns specific cluster IDs in permissions

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel has per-cluster bindings, the middleware must produce specific IDs (not wildcard) for cluster permissions |

**Fixtures:**
- `inputSensitiveMock` with `authorizedIDs` returning `["cluster-a", "cluster-b"]` for cluster resource type; `allowedTuples` set for node and project

**Steps:**
- **Given** `ListAuthorizedResources` for cluster returns `["cluster-a", "cluster-b"]`
- **When** the middleware runs for `user-1` in `org-1`
- **Then** `permissions["openshift.cluster"]` contains the specific cluster IDs

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` equals `["cluster-a", "cluster-b"]`

---

### UT-MW-SLO-002: ListAuthorizedResources returns specific project IDs in permissions

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Same as SLO-001 but for project resource type |

**Fixtures:**
- `inputSensitiveMock` with `authorizedIDs` returning `["ns-a", "ns-b"]` for project resource type; `allowedTuples` set for node and cluster

**Steps:**
- **Given** `ListAuthorizedResources` for project returns `["ns-a", "ns-b"]`
- **When** the middleware runs for `user-1` in `org-1`
- **Then** `permissions["openshift.project"]` contains the specific project IDs

**Acceptance Criteria:**
- `permissions["openshift.project"]` equals `["ns-a", "ns-b"]`

---

### UT-MW-SLO-003: Empty list + Check allowed -> wildcard fallback

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When no specific resources are found but tenant-level access is granted, the user gets wildcard access |

**Fixtures:**
- `inputSensitiveMock` with empty `authorizedIDs` for cluster and `allowedTuples[cluster_read]=true`

**Steps:**
- **Given** `ListAuthorizedResources` for cluster returns empty and `CheckPermission` for cluster returns `true`
- **When** the middleware runs for `user-1` in `org-1`
- **Then** `permissions["openshift.cluster"]` is set to wildcard

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` equals `["*"]`

---

### UT-MW-SLO-004: Empty list + Check denied -> no permission for that type

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When neither resource-level nor tenant-level access exists, the permission must be absent |

**Fixtures:**
- `inputSensitiveMock` with empty `authorizedIDs` and `allowedTuples[cluster_read]=false`

**Steps:**
- **Given** `ListAuthorizedResources` for cluster returns empty and `CheckPermission` for cluster returns `false`
- **When** the middleware runs for `user-1` in `org-1`
- **Then** `permissions["openshift.cluster"]` is absent or empty

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` is empty or absent

---

### UT-MW-SLO-005: Node always uses CheckPermission (no LookupResources)

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Node permissions are authorization-only (no data filtering); they always produce wildcard |

**Fixtures:**
- `inputSensitiveMock` with `allowedTuples[node_read]=true`; no `authorizedIDs` entry for node

**Steps:**
- **Given** `CheckPermission` for node returns `true`
- **When** the middleware runs
- **Then** `permissions["openshift.node"]` equals `["*"]`

**Acceptance Criteria:**
- `permissions["openshift.node"]` equals `["*"]`

---

### UT-MW-SLO-006: List error + Check fallback succeeds -> wildcard

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Graceful degradation when the Lookup service is down |

**Fixtures:**
- `inputSensitiveMock` with `listErrors[cluster]` returning `codes.Unavailable`; `allowedTuples[cluster_read]=true`

**Steps:**
- **Given** `ListAuthorizedResources` for cluster returns a gRPC error
- **And** `CheckPermission` for cluster returns `true`
- **When** the middleware runs
- **Then** `permissions["openshift.cluster"]` equals `["*"]`

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` equals `["*"]`

---

### UT-MW-SLO-007: List error + Check error -> no permission

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | When both lookup and check fail, access is denied (fail-closed) |

**Fixtures:**
- `inputSensitiveMock` with `listErrors[cluster]` and `errorTuples[cluster_read]` both returning errors

**Steps:**
- **Given** both `ListAuthorizedResources` and `CheckPermission` for cluster return errors
- **When** the middleware runs
- **Then** `permissions["openshift.cluster"]` is absent or empty

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` is empty or absent

---

### UT-MW-SLO-008: Mixed scenario - cluster IDs + project wildcard + node check

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Validates that different permission types can use different resolution paths simultaneously |

**Fixtures:**
- `inputSensitiveMock` with:
  - `authorizedIDs[cluster|read]` = `["cluster-a", "cluster-b"]`
  - `authorizedIDs[project|read]` = `[]` (empty); `allowedTuples[project_read]` = `true`
  - `allowedTuples[node_read]` = `true`

**Steps:**
- **Given** cluster has specific resource bindings, project has no bindings but tenant-level access, node has tenant-level access
- **When** the middleware runs
- **Then** `permissions["openshift.cluster"]` = `["cluster-a", "cluster-b"]`, `permissions["openshift.project"]` = `["*"]`, `permissions["openshift.node"]` = `["*"]`

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` contains exactly 2 specific IDs
- `permissions["openshift.project"]` equals `["*"]`
- `permissions["openshift.node"]` equals `["*"]`

---

### UT-MW-SLO-009: Only node allowed -> 1 permission entry

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | When only node access is granted, the user can still access the API but with minimal scope |

**Fixtures:**
- `inputSensitiveMock` with only `allowedTuples[node_read]=true`; all cluster/project list and check return empty/false

**Steps:**
- **Given** only node access is granted via `CheckPermission`
- **When** the middleware runs
- **Then** only `permissions["openshift.node"]` is set to `["*"]`; cluster and project are absent

**Acceptance Criteria:**
- `permissions["openshift.node"]` equals `["*"]`
- `len(permissions)` equals 1

---

### UT-MW-AUTH-001: User with full OpenShift access sees all resource types

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | An admin-level user with all 3 OpenShift permissions must be able to view recommendations for all clusters, nodes, and projects |

**Fixtures:**
- Input-sensitive mock allows all 3 permissions for `("org-1", *, "admin")`
- Echo context with Identity: `OrgID: "org-1"`, `User.Username: "admin"`
- Handler counter to verify invocation

**Steps:**
- **Given** a user `admin` in `org-1` with all 3 OpenShift Kessel permissions
- **When** the Kessel middleware processes the request
- **Then** `user.permissions` contains exactly 3 keys, each with value `["*"]`, and the handler is invoked

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(3))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.node"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-MW-AUTH-002: User with cluster-only access sees only cluster data

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user authorized for clusters but not nodes or projects must only see cluster-level recommendations |

**Fixtures:**
- Input-sensitive mock allows only `("org-1", "cost_management_openshift_cluster_read", "viewer")`

**Steps:**
- **Given** a user `viewer` with only `cluster_read` permission in Kessel
- **When** the Kessel middleware processes the request
- **Then** `user.permissions` contains exactly 1 key `"openshift.cluster"` with value `["*"]`

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(1))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-MW-AUTH-003: User with node and project access

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Partial permission combinations must be correctly represented in the permissions map |

**Fixtures:**
- Input-sensitive mock allows node and project for `("org-1", *, "partial")`

**Steps:**
- **Given** a user `partial` with `node_read` and `project_read` but not `cluster_read`
- **When** the Kessel middleware processes the request
- **Then** `user.permissions` contains exactly 2 keys

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(2))`
- `Expect(permissions["openshift.node"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-MW-AUTH-004: User with no permissions is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user with no Kessel permissions must receive HTTP 403, not an empty successful response |

**Fixtures:**
- Input-sensitive mock allows nothing
- Handler invocation counter initialized to 0

**Steps:**
- **Given** a user `nobody` with no Kessel permissions in `org-1`
- **When** the Kessel middleware processes the request
- **Then** the response is HTTP 403 and the handler is never invoked

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"` (matches RBAC middleware format, see 1.9)
- `Expect(handlerCallCount).To(Equal(0))`

---

### UT-MW-AUTH-005: Unauthenticated request is rejected

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A request without identity information must be rejected before any Kessel calls are made |

**Fixtures:**
- No Identity set in Echo context

**Steps:**
- **Given** a request arrives without an `X-Rh-Identity` header (no Identity in context)
- **When** the Kessel middleware processes the request
- **Then** the response is HTTP 401

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusUnauthorized))`
- Response body contains `"unauthorized"`

---

### UT-MW-AUTH-006: Identity missing org_id is rejected

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | An identity without an organization ID cannot be authorized; the system must fail clearly |

**Fixtures:**
- Identity with `OrgID: ""`, `User.Username: "user-1"`

**Steps:**
- **Given** the identity has an empty `org_id`
- **When** the Kessel middleware processes the request
- **Then** the response is HTTP 401

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusUnauthorized))`
- Response body contains `"unauthorized"`

---

### UT-MW-AUTH-007: Identity missing username is rejected

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | An identity without a username cannot be mapped to a Kessel principal; the system must reject it |

**Fixtures:**
- Identity with `OrgID: "org-1"`, `User.Username: ""`

**Steps:**
- **Given** the identity has an empty `username`
- **When** the Kessel middleware processes the request
- **Then** the response is HTTP 401

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusUnauthorized))`
- Response body contains `"unauthorized"`

---

### UT-MW-AUTH-008: Partial Kessel failure does not block available permissions

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | If one of three Kessel checks fails (e.g., cluster check times out), the user should still get access to the resources that were successfully checked |

**Fixtures:**
- Input-sensitive mock returns error for cluster, ALLOWED for node and project

**Steps:**
- **Given** Kessel returns a gRPC error for the `cluster_read` check but succeeds for `node_read` and `project_read`
- **When** the Kessel middleware processes the request
- **Then** `user.permissions` contains 2 keys (node and project), and the handler is invoked

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(2))`
- `Expect(permissions["openshift.node"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-MW-AUTH-009: Total Kessel failure denies access (fail-closed, not 5xx)

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When Kessel is completely unavailable, the user must be denied access (fail-closed) with HTTP 403, not a 500 server error. This matches ros-ocp-backend's existing RBAC behavior (see 1.9). |

**Fixtures:**
- Input-sensitive mock returns error for all 3 checks

**Steps:**
- **Given** all 3 Kessel CheckPermission calls return gRPC errors
- **When** the Kessel middleware processes the request
- **Then** the response is HTTP 403 (not 500)

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"` (matches RBAC middleware format, see 1.9)

---

### UT-MW-BACKEND-001: Server selects correct middleware based on config flags

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The `RBAC_ENABLE` + `AUTHORIZATION_BACKEND` combination must control which middleware is wired into the HTTP pipeline; a misconfiguration here silently bypasses all authorization |

**Fixtures:**
- Extracted middleware-registration helper function
- Four config combinations

**Steps:**
- **Given** `RBACEnabled=true` and `AuthorizationBackend="kessel"`
- **When** the server registers middleware
- **Then** the Kessel middleware is registered and the RBAC middleware is not
- **Given** `RBACEnabled=true` and `AuthorizationBackend="rbac"`
- **When** the server registers middleware
- **Then** the RBAC middleware is registered and the Kessel middleware is not
- **Given** `RBACEnabled=true` and `AuthorizationBackend="foobar"` (unrecognized value)
- **When** the server registers middleware
- **Then** the RBAC middleware is registered (the `default` clause in the switch falls back to RBAC)
- **Given** `RBACEnabled=false` (any `AuthorizationBackend` value)
- **When** the server registers middleware
- **Then** no authorization middleware is registered

**Acceptance Criteria:**
- Four sub-cases verified in a single test with `subTest`-style assertions
- Each sub-case verifies the correct middleware type is returned (or none)

---

### UT-RBAC-PERM-001: Unrestricted cluster access grants wildcard

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When RBAC returns a cluster permission without resource definitions, the user has access to all clusters |

**Fixtures:**
- ACL: `Permission: "cost-management:openshift.cluster:read"`, empty `ResourceDefinitions`

**Steps:**
- **Given** an RBAC ACL grants `openshift.cluster:read` with no resource restrictions
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map has 1 key with a wildcard value

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(1))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`

---

### UT-RBAC-PERM-002: Cluster access scoped to single UUID

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When RBAC restricts access to a specific cluster UUID, only that cluster's recommendations are visible |

**Fixtures:**
- ACL with `ResourceDefinitions: [{AttributeFilter: {Value: "uuid-abc"}}]`

**Steps:**
- **Given** an RBAC ACL grants `openshift.cluster:read` scoped to UUID `uuid-abc`
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map contains exactly `["uuid-abc"]` for `openshift.cluster`

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"uuid-abc"}))`

---

### UT-RBAC-PERM-003: Cluster access scoped to multiple UUIDs

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Users with access to multiple specific clusters must see all of them |

**Fixtures:**
- ACL with `ResourceDefinitions: [{AttributeFilter: {Value: []interface{}{"uuid-1", "uuid-2"}}}]`

**Steps:**
- **Given** an RBAC ACL grants cluster access scoped to 2 UUIDs
- **When** `aggregate_permissions` processes the ACL
- **Then** both UUIDs are present

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"uuid-1", "uuid-2"}))`

---

### UT-RBAC-PERM-004: Project access scoped to single namespace

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Users with namespace-scoped access must only see recommendations for those namespaces |

**Fixtures:**
- ACL: `Permission: "cost-management:openshift.project:read"`, `Value: "my-namespace"`

**Steps:**
- **Given** an RBAC ACL grants project access scoped to namespace `my-namespace`
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map contains `["my-namespace"]` for `openshift.project`

**Acceptance Criteria:**
- `Expect(permissions["openshift.project"]).To(Equal([]string{"my-namespace"}))`

---

### UT-RBAC-PERM-005: Unrestricted node access grants wildcard

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Unrestricted node access means the user can see node-level recommendations across all nodes |

**Fixtures:**
- ACL: `Permission: "cost-management:openshift.node:read"`, empty `ResourceDefinitions`

**Steps:**
- **Given** an RBAC ACL grants `openshift.node:read` with no restrictions
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map has `["*"]` for `openshift.node`

**Acceptance Criteria:**
- `Expect(permissions["openshift.node"]).To(Equal([]string{"*"}))`

---

### UT-RBAC-PERM-006: Wildcard resource type grants global access

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A wildcard ACL (`*`) means the user has access to everything -- this is the admin path |

**Fixtures:**
- ACL: `Permission: "cost-management:*:read"`

**Steps:**
- **Given** an RBAC ACL grants wildcard (`*`) access
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map has key `"*"` with an empty slice

**Acceptance Criteria:**
- `Expect(permissions["*"]).To(Equal([]string{}))`

---

### UT-RBAC-PERM-007: Non-openshift types do not grant any access

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Permissions for `cost_model` or other non-OpenShift types must not grant access to OpenShift recommendations |

**Fixtures:**
- ACL: `Permission: "cost-management:cost_model:write"`

**Steps:**
- **Given** an RBAC ACL grants only `cost_model:write`
- **When** `aggregate_permissions` processes the ACL
- **Then** the returned map is empty

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(0))`

---

### UT-RBAC-PERM-008: No ACLs yields no permissions

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | An empty RBAC response must not be misinterpreted as "full access" |

**Fixtures:**
- Empty ACL list: `[]types.RbacData{}`

**Steps:**
- **Given** RBAC returns no ACLs
- **When** `aggregate_permissions` processes the empty list
- **Then** the returned map is empty

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(0))`

---

### UT-RBAC-PERM-009: Multiple ACLs for same type accumulate scopes

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Multiple RBAC roles granting access to different clusters must combine to give the user access to all of them |

**Fixtures:**
- Two ACLs for `openshift.cluster:read` with string values `"uuid-1"` and `"uuid-2"`

**Steps:**
- **Given** two separate RBAC ACLs grant cluster access to different UUIDs
- **When** `aggregate_permissions` processes both ACLs
- **Then** the returned map accumulates both UUIDs

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"uuid-1", "uuid-2"}))`

---

### UT-RBAC-PERM-010: Mixed types from single RBAC response

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | A user with permissions across multiple resource types must see the correct access map for each type |

**Fixtures:**
- ACLs: cluster (wildcard), project (string `"ns-1"`), and wildcard `*`

**Steps:**
- **Given** an RBAC response contains ACLs for cluster, project, and wildcard types
- **When** `aggregate_permissions` processes the response
- **Then** all three types are represented with correct values

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(3))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"ns-1"}))`
- `Expect(permissions["*"]).To(Equal([]string{}))`

---

### UT-RBAC-PERM-011: Multiple resource definitions on single ACL all apply

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | A single RBAC role with multiple resource definitions must grant access to all defined resources |

**Fixtures:**
- One ACL with 2 `ResourceDefinitions`: string values `"uuid-a"` and `"uuid-b"`

**Steps:**
- **Given** a single RBAC ACL has 2 resource definitions for cluster access
- **When** `aggregate_permissions` processes the ACL
- **Then** both resource IDs are present

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"uuid-a", "uuid-b"}))`

---

### UT-RBAC-PERM-012: Extra colons in permission string do not break parsing

| Field | Value |
|-------|-------|
| Priority | P3 (Low) |
| Business Value | Defensively handle malformed permission strings without crashing |

**Fixtures:**
- ACL: `Permission: "app:openshift.cluster:read:extra"` (4 colon-separated segments)

**Steps:**
- **Given** an RBAC ACL has a permission string with extra colons
- **When** `aggregate_permissions` processes the ACL
- **Then** the second segment (`openshift.cluster`) is correctly extracted

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`

---

### UT-RBAC-PERM-013: Permission string with no colons is gracefully skipped

| Field | Value |
|-------|-------|
| Priority | P3 (Low) |
| Business Value | A malformed permission string (no colons) must be skipped without panicking; the resulting permissions map should be empty for that ACL |

**Fixtures:**
- ACL: `Permission: "openshift"` (no colons)

**Steps:**
- **Given** an RBAC ACL has a permission string with no colons
- **When** `aggregate_permissions` processes the ACL
- **Then** it returns an empty permissions map (the malformed entry is skipped)

**Acceptance Criteria:**
- `len(perms) == 0` (malformed permission is gracefully ignored)

---

### UT-RBAC-ACCESS-001: User with RBAC permissions can proceed

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The existing RBAC authorization path must continue to work correctly after adding Kessel support |

**Fixtures:**
- `httptest` server returning RBAC response with `openshift.cluster:read` wildcard ACL
- Echo context with `X-Rh-Identity` header

**Steps:**
- **Given** the RBAC API returns a valid permission for `openshift.cluster:read`
- **When** the RBAC middleware processes the request
- **Then** `user.permissions` contains the correct cluster wildcard and the handler is invoked

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-RBAC-ACCESS-002: User with empty RBAC response is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | If RBAC returns no permissions, the user must be denied |

**Fixtures:**
- `httptest` server returning `{"data": []}`

**Steps:**
- **Given** the RBAC API returns an empty data array
- **When** the RBAC middleware processes the request
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"`

---

### UT-RBAC-ACCESS-003: User with only non-openshift RBAC permissions is denied

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Permissions for non-OpenShift resources (e.g., cost_model) must not grant access to OpenShift recommendations |

**Fixtures:**
- `httptest` server returning only `cost-management:cost_model:write` ACL

**Steps:**
- **Given** the RBAC API returns only a `cost_model:write` permission
- **When** the RBAC middleware processes the request
- **Then** the response is HTTP 403 (aggregate_permissions returns an empty map)

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"`

---

### UT-RBAC-ACCESS-004: Paginated RBAC response includes all pages

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Users with many RBAC roles must not lose permissions due to pagination truncation |

**Fixtures:**
- `httptest` server: page 1 returns cluster perm + `Links.Next`; page 2 returns project perm

**Steps:**
- **Given** the RBAC API response spans 2 pages
- **When** the RBAC middleware processes the request
- **Then** permissions from both pages are combined

**Acceptance Criteria:**
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### UT-RBAC-ACCESS-005: RBAC API server error denies access (fail-closed)

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When the RBAC API returns an HTTP error status, users must be denied rather than granted access |

**Fixtures:**
- `httptest` server returning HTTP 500

**Steps:**
- **Given** the RBAC API returns an HTTP 500 error (server is reachable but returning errors)
- **When** the RBAC middleware processes the request
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"`

---

### UT-RBAC-ACCESS-006: RBAC API network failure denies access gracefully

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | When the RBAC API host is completely unreachable, the middleware must fail closed with HTTP 403 instead of crashing. |

**Fixtures:**
- `httptest` server that is closed/stopped before the request (simulates network failure)

**Steps:**
- **Given** the RBAC API host is completely unreachable (network failure, not HTTP error)
- **When** the RBAC middleware processes the request
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `rec.Code == http.StatusForbidden`

---

**Note on UT-SRC-APPID scenarios:** Scenarios UT-SRC-APPID-001 through -004 drive **new code** (the `COST_APPLICATION_TYPE_ID` env var injection path that will be added to `GetCostApplicationID()`). Scenarios UT-SRC-APPID-005 through -009 cover **existing + modified behavior** (empty/missing env var falling through to the existing HTTP path). The current `GetCostApplicationID()` in `internal/utils/sources/sources_api.go` has no env var check -- it always calls the Sources API.

---

### UT-SRC-APPID-001: Helm-injected ID is used directly

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | When the Helm chart injects COST_APPLICATION_TYPE_ID, the housekeeper must use it without calling the Sources API |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID=5` env var set

**Steps:**
- **Given** the `COST_APPLICATION_TYPE_ID` environment variable is set to `"5"`
- **When** `GetCostApplicationID()` is called
- **Then** it returns `5, nil`

**Acceptance Criteria:**
- `Expect(id).To(Equal(5))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-SRC-APPID-002: Zero is a valid injected ID

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | The default Helm value `"0"` must be accepted without error |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID=0` env var set

**Steps:**
- **Given** `COST_APPLICATION_TYPE_ID` is `"0"`
- **When** `GetCostApplicationID()` is called
- **Then** it returns `0, nil`

**Acceptance Criteria:**
- `Expect(id).To(Equal(0))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-SRC-APPID-003: Large injected ID is accepted

| Field | Value |
|-------|-------|
| Priority | P3 (Low) |
| Business Value | The env var path must handle any valid integer, not just small values |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID=99999` env var set

**Steps:**
- **Given** `COST_APPLICATION_TYPE_ID` is `"99999"`
- **When** `GetCostApplicationID()` is called
- **Then** it returns `99999, nil`

**Acceptance Criteria:**
- `Expect(id).To(Equal(99999))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-SRC-APPID-004: Non-numeric injected ID reports clear error

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | A misconfigured Helm value must produce a clear error message, not a silent failure or panic |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID=abc` env var set

**Steps:**
- **Given** `COST_APPLICATION_TYPE_ID` is set to a non-numeric value
- **When** `GetCostApplicationID()` is called
- **Then** it returns `0` and an error describing the parse failure

**Acceptance Criteria:**
- `Expect(id).To(Equal(0))`
- `Expect(err).To(HaveOccurred())`
- `Expect(err.Error()).To(ContainSubstring("invalid"))` (tests the new wrapper message, not Go internals)

---

### UT-SRC-APPID-005: Empty env var fetches from Sources API

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | An empty env var must be treated as "not set" and trigger the HTTP fallback to Sources API |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID=""` env var set
- `httptest` server returns `{"data": [{"id": "7"}]}`

**Steps:**
- **Given** `COST_APPLICATION_TYPE_ID` is set to an empty string
- **When** `GetCostApplicationID()` is called
- **Then** it fetches from the Sources API and returns `7, nil`

**Acceptance Criteria:**
- `Expect(id).To(Equal(7))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-SRC-APPID-006: Missing env var fetches from Sources API

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Without the Helm injection, the system must fall back to the Sources API (backward compatibility) |

**Fixtures:**
- `COST_APPLICATION_TYPE_ID` env var unset
- `httptest` server returns `{"data": [{"id": "3"}]}`

**Steps:**
- **Given** `COST_APPLICATION_TYPE_ID` is not set
- **When** `GetCostApplicationID()` is called
- **Then** it fetches from the Sources API and returns `3, nil`

**Acceptance Criteria:**
- `Expect(id).To(Equal(3))`
- `Expect(err).ToNot(HaveOccurred())`

---

### UT-SRC-APPID-007: Sources API returns 404

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | A missing Sources API endpoint must produce an error, not a panic |

**Fixtures:**
- Env var unset; `httptest` server returns 404

**Steps:**
- **Given** the Sources API returns HTTP 404
- **When** `GetCostApplicationID()` is called
- **Then** it returns `0` and an error

**Acceptance Criteria:**
- `Expect(id).To(Equal(0))`
- `Expect(err).To(HaveOccurred())`

---

### UT-SRC-APPID-008: Sources API returns malformed JSON

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | A corrupted Sources API response must produce a clear error, not a panic |

**Fixtures:**
- Env var unset; `httptest` server returns `not-json`

**Steps:**
- **Given** the Sources API returns invalid JSON
- **When** `GetCostApplicationID()` is called
- **Then** it returns `0` and an error

**Acceptance Criteria:**
- `Expect(id).To(Equal(0))`
- `Expect(err).To(HaveOccurred())`
- The error message comes from the existing wrapper (`"unable to unmarshal..."`); asserting on the wrapper text couples the test to the message string. Prefer `HaveOccurred()` here.

---

### UT-SRC-APPID-009: Sources API returns empty data array returns error gracefully

| Field | Value |
|-------|-------|
| Priority | P2 (Medium) |
| Business Value | When the Sources API returns a valid JSON response with an empty `data` array, `GetCostApplicationID()` must return a graceful error instead of crashing. |

**Fixtures:**
- Env var unset; `httptest` server returns `{"data": []}`

**Steps:**
- **Given** the Sources API returns a valid JSON response with an empty `data` array
- **When** `GetCostApplicationID()` is called
- **Then** an error is returned and ID is 0

**Acceptance Criteria:**
- `err != nil`
- `id == 0`

---

## 3. Tier 2 -- Integration Tests (IT)

Build tag: `//go:build integration`

Precondition: SpiceDB + Kessel Relations API running via Podman Compose, ZED schema loaded, test data seeded via Kessel Relations API `CreateTuples`.

**Handler stub:** IT-MW-AUTH-006 and IT-MW-AUTH-007 test end-to-end HTTP flow through identity + Kessel middleware + handler. To avoid requiring a database, these tests register a **stub handler** that returns HTTP 200 with a static JSON body when reached. This isolates the test to the middleware pipeline. The stub handler is shared across all IT scenarios that test end-to-end HTTP.

---

### IT-MW-AUTH-001: Authorized principal gets full access

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | An authorized user must receive all 3 OpenShift permission wildcards when their role binding covers all 3 types |

**Fixtures:**
- SpiceDB seeded: role binding grants all 3 read permissions to `principal:user-1` in `tenant:org-1`
- Echo server with Identity + Kessel middleware

**Steps:**
- **Given** `user-1` has a Kessel role binding granting all 3 OpenShift read permissions in `org-1`
- **When** the Kessel middleware processes a request for `org-1`/`user-1`
- **Then** `user.permissions` has 3 keys, each with value `["*"]`

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(3))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.node"]).To(Equal([]string{"*"}))`
- `Expect(permissions["openshift.project"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### IT-MW-AUTH-002: Unauthorized principal is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user without any Kessel role bindings must receive HTTP 403 |

**Fixtures:**
- SpiceDB seeded: no role bindings for `user-2` in `org-1`

**Steps:**
- **Given** `user-2` has no Kessel role bindings in `org-1`
- **When** the Kessel middleware processes a request for `org-1`/`user-2`
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`
- Response body contains `"User is not authorized"`

---

### IT-MW-AUTH-003: Partial binding yields partial access

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user with only cluster_read binding must see only cluster recommendations, not node or project |

**Fixtures:**
- SpiceDB seeded: only `cluster_read` binding for `user-3` in `org-1`

**Steps:**
- **Given** `user-3` has only `cost_management_openshift_cluster_read` in `org-1`
- **When** the Kessel middleware processes the request
- **Then** `user.permissions` has exactly 1 key

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(1))`
- `Expect(permissions["openshift.cluster"]).To(Equal([]string{"*"}))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### IT-MW-AUTH-004: Group membership propagates to individual

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Users added to a Kessel group must inherit the group's role bindings without explicit per-user bindings |

**Fixtures:**
- SpiceDB seeded: `user-4` is member of `group-1`; `group-1` has role binding granting all 3 permissions in `org-1`

**Steps:**
- **Given** `user-4` is a member of `group-1`, and `group-1` has all 3 OpenShift permissions
- **When** the Kessel middleware processes a request for `user-4`
- **Then** `user.permissions` has 3 keys

**Acceptance Criteria:**
- `Expect(permissions).To(HaveLen(3))`
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### IT-MW-AUTH-005: Permissions do not leak across tenants

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A user authorized in one organization must not gain access in another -- this is a core multi-tenancy security invariant |

**Fixtures:**
- SpiceDB seeded: `user-5` has full access in `org-2`, no bindings in `org-1`

**Steps:**
- **Given** `user-5` is authorized in `org-2` but not `org-1`
- **When** the Kessel middleware processes a request for `org-1`/`user-5`
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`

---

### IT-MW-AUTH-006: Authorized end-to-end HTTP request succeeds

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A complete HTTP request through identity + Kessel middleware + stub handler must succeed for an authorized user |

**Fixtures:**
- Full Echo server with identity middleware + Kessel middleware + stub handler (returns 200 with static JSON)
- SpiceDB seeded for `user-6`/`org-3`

**Steps:**
- **Given** `user-6` is fully authorized in `org-3`
- **When** GET `/api/cost-management/v1/recommendations/openshift` with valid `X-Rh-Identity`
- **Then** the response is HTTP 200

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusOK))`

---

### IT-MW-AUTH-007: Unauthorized end-to-end HTTP request is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A complete HTTP request for an unauthorized user must return HTTP 403 before reaching the handler |

**Fixtures:**
- Same Echo server with stub handler; no SpiceDB bindings for `user-7`/`org-3`

**Steps:**
- **Given** `user-7` has no bindings in `org-3`
- **When** same GET request with `user-7`'s identity
- **Then** the response is HTTP 403

**Acceptance Criteria:**
- `Expect(rec.Code).To(Equal(http.StatusForbidden))`

---

### IT-MW-AUTH-008: Sequential requests do not share authorization state

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Authorization state from one request must not leak into the next -- a bug here would grant unauthorized users access |

**Fixtures:**
- Same Echo server; SpiceDB has bindings for `user-6` but not `user-7`

**Steps:**
- **Given** `user-6` is authorized and `user-7` is not
- **When** two sequential requests are sent to the same Echo instance: first `user-6`, then `user-7`
- **Then** first returns HTTP 200, second returns HTTP 403

**Acceptance Criteria:**
- `Expect(firstRec.Code).To(Equal(http.StatusOK))`
- `Expect(secondRec.Code).To(Equal(http.StatusForbidden))`

---

### IT-MW-BACKEND-001: Switching backend config changes authorization behavior

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The `RBAC_ENABLE` + `AUTHORIZATION_BACKEND` combination must control which authorization system is active at runtime; this is the highest-risk integration point |

**Fixtures:**
- Two Echo server configurations, both with `RBACEnabled=true`:
  - Config A: `AuthorizationBackend=kessel` (Kessel middleware, SpiceDB seeded for `user-9`/`org-5`)
  - Config B: `AuthorizationBackend=rbac` (RBAC middleware, `httptest` server returning 403 for all users)

**Steps:**
- **Given** `user-9` is authorized in Kessel but not in the RBAC `httptest` server
- **When** Config A (Kessel) processes the request
- **Then** the response is HTTP 200 (Kessel grants access)
- **When** Config B (RBAC) processes the same request
- **Then** the response is HTTP 403 (RBAC denies access)

**Acceptance Criteria:**
- `Expect(kesselRec.Code).To(Equal(http.StatusOK))`
- `Expect(rbacRec.Code).To(Equal(http.StatusForbidden))`

---

### IT-MW-SLO-001: LookupResources returns specific cluster IDs through full stack

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | With real SpiceDB + Relations API, verify that LookupResources returns actual resource IDs seeded via workspace+resource tuples |

**Acceptance Criteria:**
- `permissions["openshift.cluster"]` contains >= 2 cluster IDs (e.g., `it-cluster-a`, `it-cluster-b`)

---

### IT-MW-SLO-002: LookupResources returns specific project IDs through full stack

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Same as SLO-001 for project resources |

**Fixtures:**
- Workspace `ws-it1` with project resources `it-project-a`, `it-project-b` linked via `t_workspace`
- User `it-user-1` has project_read via role binding

**Steps:**
- **Given** `it-user-1` has project_read access in `org-it1` and project resources are bound
- **When** the middleware calls `LookupResources(cost_management/openshift_project, read)`
- **Then** `permissions["openshift.project"]` contains `["it-project-a", "it-project-b"]`

**Acceptance Criteria:**
- `permissions["openshift.project"]` contains >= 2 project IDs

---

### IT-MW-SLO-003: Node permissions are always wildcard (no LookupResources)

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Verify node permissions use CheckPermission, not LookupResources, in the full stack |

**Fixtures:**
- User `it-user-1` has node_read via tenant-level role binding in `org-it1`

**Steps:**
- **Given** `it-user-1` has node_read access in `org-it1`
- **When** the middleware runs
- **Then** `permissions["openshift.node"]` equals `["*"]`

**Acceptance Criteria:**
- `permissions["openshift.node"]` equals `["*"]`

---

### IT-MW-SLO-004: Unbound user gets no results from LookupResources -> HTTP 403

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | User with no bindings should get empty LookupResources results and be denied |

**Fixtures:**
- User `it-user-unbound` has no role bindings in any tenant

**Steps:**
- **Given** `it-user-unbound` has no bindings
- **When** the middleware runs
- **Then** HTTP 403 is returned with `"User is not authorized"`

**Acceptance Criteria:**
- `rec.Code == 403`

---

### IT-MW-COMBINED-001: Combined cluster + project + node permissions in a single request

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Verifies that a single middleware invocation simultaneously resolves specific cluster IDs, specific project IDs, and wildcard node access into one coherent `user.permissions` map. This is the closest scenario to production behavior where `AddRBACFilter` receives all three dimensions together. |

**Fixtures:**
- Kessel stack running, `seedTestData()` completed
- `user-1` in `org-1` with all 3 permissions via `binding-1`
- Workspace `ws-org1` with resources: `it-cluster-a`, `it-cluster-b` (clusters), `it-proj-x`, `it-proj-y` (projects)

**Steps:**
1. `GIVEN` user-1/org-1 identity header
2. `WHEN` a single GET /test request passes through `KesselMiddleware`
3. `THEN` HTTP status is 200
4. `AND` `user.permissions` has exactly 3 keys
5. `AND` `openshift.cluster` contains exactly `{"it-cluster-a", "it-cluster-b"}` (order-independent, no wildcard)
6. `AND` `openshift.project` contains exactly `{"it-proj-x", "it-proj-y"}` (order-independent, no wildcard)
7. `AND` `openshift.node` equals `["*"]`

**Acceptance Criteria:**
- `rec.Code == 200`
- `len(perms) == 3`
- `openshift.cluster` contains exactly `it-cluster-a` and `it-cluster-b` (no duplicates, no `*`)
- `openshift.project` contains exactly `it-proj-x` and `it-proj-y` (no duplicates, no `*`)
- `openshift.node` equals `["*"]`

---

### IT-MW-PARITY-001: RBAC and Kessel permission shapes are identical

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The Kessel middleware must produce `user.permissions` in exactly the same `map[string][]string` shape as RBAC; any mismatch breaks downstream query filtering |

**Acceptance Criteria:**
- `permissions` has exactly 3 keys: `openshift.cluster`, `openshift.node`, `openshift.project`
- Each value is either `["*"]` or a non-empty list of specific IDs
- No empty string values

---

## 4. Tier 3 -- Contract Tests (CT)

Build tag: `//go:build contract`

Precondition: SpiceDB running via Podman Compose with ZED schema loaded via `SPICEDB_SCHEMA_FILE` volume mount. Tests seed data via Kessel Relations API `CreateTuples` and call `Check` and `LookupResources` via Kessel Relations API gRPC stubs. No application middleware or HTTP involved.

---

### CT-KESSEL-SCHEMA-001: cluster_read permission resolves through role binding

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The ZED schema must correctly resolve `cost_management_openshift_cluster_read` through the tenant -> role_binding -> role chain; if this fails, no user can be authorized for cluster access |

**Fixtures:**
- SpiceDB seeded via Kessel Relations API `CreateTuples`:
  - Role `ct-role-1` with relation `t_cost_management_openshift_cluster_read` -> `rbac/principal:*`
  - Role binding `ct-binding-1`: `t_granted` -> `ct-role-1`, `t_subject` -> `rbac/principal:ct-user-1`
  - Tenant `org-ct1`: `t_default_binding` -> `ct-binding-1`

**Steps:**
- **Given** `ct-user-1` has a role binding granting `cluster_read` in `org-ct1`
- **When** `Check(resource=rbac/tenant:org-ct1, relation=cost_management_openshift_cluster_read, subject=rbac/principal:ct-user-1)` is called
- **Then** the response is `ALLOWED_TRUE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_TRUE))`

---

### CT-KESSEL-SCHEMA-002: node_read permission resolves through role binding

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The ZED schema must correctly resolve `cost_management_openshift_node_read` |

**Fixtures:**
- Same pattern as CT-KESSEL-SCHEMA-001, with `t_cost_management_openshift_node_read`

**Steps:**
- **Given** `ct-user-1` has a role binding granting `node_read` in `org-ct1`
- **When** `Check(relation=cost_management_openshift_node_read)` is called
- **Then** the response is `ALLOWED_TRUE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_TRUE))`

---

### CT-KESSEL-SCHEMA-003: project_read permission resolves through role binding

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The ZED schema must correctly resolve `cost_management_openshift_project_read` |

**Fixtures:**
- Same pattern, with `t_cost_management_openshift_project_read`

**Steps:**
- **Given** `ct-user-1` has a role binding granting `project_read` in `org-ct1`
- **When** `Check(relation=cost_management_openshift_project_read)` is called
- **Then** the response is `ALLOWED_TRUE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_TRUE))`

---

### CT-KESSEL-REL-001: Unbound principal is denied

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A principal without any role binding must be denied; SpiceDB must not default to allow |

**Fixtures:**
- No role bindings for `ct-user-2` in `org-ct1`

**Steps:**
- **Given** `ct-user-2` has no role bindings in `org-ct1`
- **When** `Check(subject=rbac/principal:ct-user-2)` is called
- **Then** the response is `ALLOWED_FALSE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_FALSE))`

---

### CT-KESSEL-REL-002: cluster_read binding does not grant node_read

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Permissions must be isolated: having cluster_read must not imply node_read or project_read |

**Fixtures:**
- `ct-user-1` has only `cluster_read` binding

**Steps:**
- **Given** `ct-user-1` has only `cluster_read` permission in `org-ct1`
- **When** `Check(relation=cost_management_openshift_node_read, subject=...ct-user-1)` is called
- **Then** the response is `ALLOWED_FALSE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_FALSE))`

---

### CT-KESSEL-GRP-001: Group member inherits group's role binding

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Group-based authorization is the primary mechanism for on-prem deployments; if group inheritance fails, no group-managed user can access recommendations |

**Fixtures:**
- `ct-user-3` is member of `group-ct1`
- `group-ct1#member` has role binding granting `cluster_read` in `org-ct1`

**Steps:**
- **Given** `ct-user-3` is a member of `group-ct1`, and `group-ct1` has a `cluster_read` binding
- **When** `Check(subject=rbac/principal:ct-user-3)` is called
- **Then** the response is `ALLOWED_TRUE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_TRUE))`

---

### CT-KESSEL-REL-003: Role binding in org-ct1 does not grant access in org-ct2

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Tenant isolation at the SpiceDB level: permissions in one tenant must not bleed into another |

**Fixtures:**
- `ct-user-1` has full bindings in `org-ct1`, none in `org-ct2`

**Steps:**
- **Given** `ct-user-1` is authorized in `org-ct1` but not `org-ct2`
- **When** `Check(resource=rbac/tenant:org-ct2, subject=...ct-user-1)` is called
- **Then** the response is `ALLOWED_FALSE`

**Acceptance Criteria:**
- `Expect(response.Allowed).To(Equal(v1beta1.CheckResponse_ALLOWED_FALSE))`

---

### CT-KESSEL-SLO-001: LookupResources returns cluster IDs for authorized principal

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | The Relations API LookupResources must return resource IDs for resources linked to a workspace where the user has the `read` permission |

**Fixtures:**
- Workspace `ws-ct1` parented to tenant `org-ct1`, bound via `ct-binding-1`
- Resources `cluster-ct-a`, `cluster-ct-b` linked to `ws-ct1` via `t_workspace`

**Acceptance Criteria:**
- `LookupResources(cost_management/openshift_cluster, read, ct-user-1)` returns `["cluster-ct-a", "cluster-ct-b"]`

---

### CT-KESSEL-SLO-002: LookupResources returns project IDs for authorized principal

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | Same as SLO-001 for project resources |

**Fixtures:**
- Workspace `ws-ct1` with project resources `project-ct-a`, `project-ct-b` linked via `t_workspace`
- User `ct-user-1` has project_read via role binding

**Steps:**
- **Given** `ct-user-1` has project_read access and project resources are bound
- **When** `LookupResources(cost_management/openshift_project, read, ct-user-1)` is called
- **Then** the response contains `["project-ct-a", "project-ct-b"]`

**Acceptance Criteria:**
- Result contains `"project-ct-a"` and `"project-ct-b"`

---

### CT-KESSEL-SLO-003: LookupResources returns empty for unbound principal

| Field | Value |
|-------|-------|
| Priority | P0 (Critical) |
| Business Value | A principal without bindings must get empty results from LookupResources |

**Fixtures:**
- No role bindings for `ct-user-unbound`

**Steps:**
- **Given** `ct-user-unbound` has no bindings in any tenant
- **When** `LookupResources(cost_management/openshift_cluster, read, ct-user-unbound)` is called
- **Then** the response is empty

**Acceptance Criteria:**
- `len(result) == 0`

---

### CT-KESSEL-SEED-001: CreateTuples upsert is idempotent

| Field | Value |
|-------|-------|
| Priority | P1 (High) |
| Business Value | Re-seeding the same tuples via `CreateTuples(upsert=true)` must not fail or create duplicates |

**Acceptance Criteria:**
- Re-calling `SeedTuples` with the same relationships succeeds without error
- Subsequent `Check` still returns `ALLOWED_TRUE`

---

## 5. Coverage Summary

### 5.1 By Priority

| Priority | UT | IT | CT | Total |
|----------|----|----|-----|-------|
| P0 (Critical) | 31 | 12 | 10 | 53 |
| P1 (High) | 32 | 2 | 1 | 35 |
| P2 (Medium) | 12 | 0 | 0 | 12 |
| P3 (Low) | 3 | 0 | 0 | 3 |
| **Total** | **78** | **14** | **11** | **103** |

### 5.2 By Module

| Module | UT | IT | CT | Total |
|--------|----|----|-----|-------|
| CFG | 14 | 0 | 0 | 14 |
| KESSEL | 17 | 0 | 11 | 28 |
| MW | 19 | 14 | 0 | 33 |
| RBAC | 19 | 0 | 0 | 19 |
| SRC | 9 | 0 | 0 | 9 |
| **Total** | **78** | **14** | **11** | **103** |

### 5.3 By Tier

| Tier | Scenarios | Runner | Kessel backend |
|------|-----------|--------|----------------|
| UT | 78 | `go test` / Ginkgo | Input-sensitive mock |
| IT | 14 | `go test -tags integration` | Real SpiceDB + Relations API |
| CT | 11 | `go test -tags contract` | Real SpiceDB + Relations API via Podman Compose |

### 5.4 Unit Test Coverage Target

Target: >80% on all new/modified files.

Covered by UT scenarios:
- `internal/config/config.go` (new fields) -- UT-CFG-BACKEND-001 through 011, UT-CFG-INV-001 through 003 (14 scenarios)
- `internal/kessel/client.go` -- UT-KESSEL-CHECK-001 through 006, UT-KESSEL-LIST-001 through 005, UT-KESSEL-LIST-GRPC-001 through 006 (17 scenarios: authorized, denied, wrong-org, outage, validation, list IDs, list empty, list error, list validation, gRPC stream processing, nil lookup, invalid resourceType)
- `internal/api/middleware/kessel.go` -- UT-MW-AUTH-001 through 009, UT-MW-SLO-001 through 009 (18 scenarios: full/partial/denied/unauthenticated/missing-fields/partial-failure/total-failure + LookupResources-based SLO logic)
- `internal/api/server.go` (switch logic) -- UT-MW-BACKEND-001 (1 scenario: 4 sub-cases)
- `internal/api/middleware/rbac.go` -- UT-RBAC-PERM-001 through 013, UT-RBAC-ACCESS-001 through 006 (19 scenarios: all aggregate_permissions branches + middleware HTTP tests + existing bugs)
- `internal/utils/sources/sources_api.go` -- UT-SRC-APPID-001 through 009 (9 scenarios: env var, HTTP fetch, error cases, 404, malformed JSON, empty data array)

### 5.5 Integration Test Coverage

Covered by IT scenarios:
- Full Kessel middleware flow -- IT-MW-AUTH-001 through 008 (8 scenarios: authorized, denied, partial, group, cross-tenant, E2E HTTP with stub handler, context isolation)
- LookupResources SLO flow -- IT-MW-SLO-001 through 004 (4 scenarios: cluster IDs, project IDs, node wildcard, unbound user 403)
- Combined dimensions -- IT-MW-COMBINED-001 (1 scenario: cluster IDs + project IDs + node wildcard in single request)
- RBAC parity -- IT-MW-PARITY-001 (1 scenario: permission map shape matches RBAC)
- Backend switch verification -- IT-MW-BACKEND-001 (1 scenario: RBAC vs Kessel middleware selection with `RBACEnabled=true` for both)

### 5.6 Contract Test Coverage

Covered by CT scenarios:
- ZED schema resolution -- CT-KESSEL-SCHEMA-001 through 003 (3 scenarios: one per permission name)
- Relations API authorization -- CT-KESSEL-REL-001 through 003 (3 scenarios: denied, permission isolation, tenant isolation)
- Group inheritance -- CT-KESSEL-GRP-001 (1 scenario: group member inherits binding)
- LookupResources SLO flow -- CT-KESSEL-SLO-001 through 003 (3 scenarios: cluster IDs, project IDs, empty for unbound principal)
- Idempotent seeding -- CT-KESSEL-SEED-001 (1 scenario: re-seeding same tuples succeeds)

### 5.7 Files Not Directly Covered by UT

- `internal/kessel/` -- new package; 100% covered by UT-KESSEL-CHECK-*, UT-KESSEL-LIST-*, CT-KESSEL-*, and CT-KESSEL-SLO-*
- `internal/testutil/kessel_seeder.go` -- test-only helper; exercised by CT and IT via `SeedTuples`/`DeleteTuples`/`Check`/`WaitForConsistency`
- `internal/api/middleware/identity.go` -- unchanged; existing tests in `identity_test.go`
- `internal/api/handlers.go` -- unchanged; exercises `user.permissions` from context (tested via IT with stub handler)
- `internal/rbac/query_builder.go` (`AddRBACFilter`, called from model layer) -- unchanged; gated on `cfg.RBACEnabled` which is `true` for both RBAC and Kessel deployments (see 1.11). For Kessel, the filter receives either `["*"]` wildcards (early-return path) or specific resource IDs from `LookupResources`. When specific IDs are returned, the same UUID/namespace filtering branches used for RBAC apply, providing resource-level access control.

### 5.8 Triage Findings Incorporated

The following findings from the post-write triage have been incorporated into this version:

| ID | Finding | Resolution |
|----|---------|------------|
| G-1/C-1 | `add_rbac_filter` gated on `RBACEnabled`; interaction with `AuthorizationBackend` | Resolved: `RBACEnabled=true` for Kessel deployments (Section 1.11); `add_rbac_filter` unchanged |
| G-2 | `openshift.node` not used in data filtering | Documented in Section 1.7 |
| G-3 | `request_user_access` nil-response panic on network failure | Fixed: UT-RBAC-ACCESS-006 verifies HTTP 403 on network failure |
| G-4 | Permission string with no colons | Captured as UT-RBAC-PERM-013 (gracefully skipped, not a bug) |
| G-5 | IT E2E tests require database | Resolved: IT uses stub handler (Section 3 header) |
| I-1 | Error message strings don't match actual code | Fixed: all 403 assertions use `"User is not authorized"` (Section 1.9) |
| I-2 | UT-RBAC-ACCESS-005 title misleading | Renamed to "RBAC API server error denies access" |
| I-3 | Priority counts wrong in Section 5.1 | Recounted from each scenario's stated priority |
| I-4 | UT-SRC-APPID-008 assertion couples to wrapper text | Changed to `HaveOccurred()` |
| C-2 | Context key contract undocumented | Documented in Section 1.13 |
| C-3 | New vs existing code in SRC scenarios | Documented in note before UT-SRC-APPID-001 |
| C-4 | UT-SRC-APPID-009 panic assertion fragile | Fixed: test now asserts graceful error return (`err != nil`, `id == 0`) |
| I-5 | CT tier infrastructure description contradicts Section 1.6 vs Section 4 | Fixed: Section 1.6 now says CT uses same Podman Compose infra as IT; added clarifying paragraph |
| M-1 | Unrecognized `AuthorizationBackend` value not tested | Added 4th sub-case to UT-MW-BACKEND-001 |
| M-2 | Config singleton reset needed for UT-CFG-BACKEND-* | Documented in new Section 1.12 |
