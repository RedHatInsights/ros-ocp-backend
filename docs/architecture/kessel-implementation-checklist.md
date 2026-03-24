# Kessel Integration — Implementation Checklist (RBAC ↔ ReBAC Parity)

**Goal:** Replace RBAC with Kessel (or vice versa) without changing business logic. Same `user.permissions` shape and semantics; same handler and `AddRBACFilter` behavior; 403 on auth failure (per ros-ocp-backend design).

**References:**
- Design: [kessel-integration.md](./kessel-integration.md) (§3 Implementation)
- Test plan: [../kessel-rebac-test-plan.md](../kessel-rebac-test-plan.md)
- Koku reference: [koku/koku_rebac/access_provider.py](https://github.com/insights-onprem/koku/blob/FLPATH-3294/kessel-rebac-integration/koku/koku_rebac/access_provider.py) (LookupResources + Check fallback)
- Kessel: [project-kessel/relations-api](https://github.com/project-kessel/relations-api) (v1beta1)

---

## Kessel project findings

| Item | Finding |
|------|---------|
| **Relations API** | The Relations API (v1beta1) exposes both **Check** and **LookupResources**. The implementation uses the Relations API for both operations via a single gRPC connection. |
| **Go client** | `github.com/project-kessel/relations-api` provides generated gRPC stubs: `KesselCheckServiceClient` and `KesselLookupServiceClient`. Both are used. |
| **Current ros-ocp-backend** | Uses **Relations API** (`relations-api` v1beta1) for both **Check** and **LookupResources**. No Inventory API client is needed for authorization. |
| **Parity** | RBAC path returns `user.permissions["openshift.cluster"]` = `["*"]` or list of IDs. Kessel path produces the same shape: LookupResources → list of IDs; when empty, Check(tenant) → `["*"]` or `[]`. |

---

## Implementation order

### 1. Config (done)

- [x] **Relations API endpoint**
  `KESSEL_RELATIONS_URL` (default `localhost:9000`) and `KESSEL_RELATIONS_CA_PATH` for TLS.

- [x] **Backend selection**
  `AUTHORIZATION_BACKEND` (`rbac` | `kessel`) and `RBAC_ENABLE` are wired; server selects middleware from config (§1.11 of test plan).

> **Note:** `KesselInventoryURL`, `KesselInventoryCAPath`, and `SpiceDBPresharedKey` config fields exist but are unused. The implementation uses Relations API for both Check and LookupResources. These fields are annotated in `config.go`.

### 2. Dependencies (done)

- [x] **Relations API**
  `github.com/project-kessel/relations-api` provides `KesselCheckServiceClient` and `KesselLookupServiceClient` (v1beta1). Both are used from a single gRPC connection.

- [x] **Remove authzed-go**
  The `authzed-go` dependency has been completely removed from `go.mod`. All test seeding and verification uses Kessel Relations API (`CreateTuples`, `Check`, `LookupResources`).

### 3. PermissionChecker interface (done)

- [x] **Extend interface**
  Added `ListAuthorizedResources(ctx, orgID, resourceType, permission, username) ([]string, error)` to `PermissionChecker` ([`client.go`](../../internal/kessel/client.go)).

- [x] **Contract**
  - For `openshift.cluster` and `openshift.project`: call `ListAuthorizedResources` (LookupResources).
  - If returned list is empty, call `CheckPermission` (workspace/tenant); if true → treat as `["*"]`, else `[]`.
  - For `openshift.node`: call `CheckPermission` only; on true → `["*"]`, else `[]`.

### 4. Kessel client implementation (done)

- [x] **Relations API connection**
  In [`internal/kessel/client.go`](../../internal/kessel/client.go), `KesselClient` wraps both `KesselCheckServiceClient` and `KesselLookupServiceClient`. Created via `NewKesselClient(checkClient, lookupClient...)`.

- [x] **LookupResources**
  `ListAuthorizedResources` builds a `LookupResourcesRequest` with resource type (e.g. `cost_management/openshift_cluster`), relation (`read`), and subject `rbac/principal:{username}`. It streams responses and collects resource IDs.

- [x] **Check fallback**
  Existing `CheckPermission` (tenant-level) is used when LookupResources returns empty. Tenant-level Check determines wildcard vs no access.

### 5. Middleware (done)

- [x] **KesselMiddleware**
  Updated middleware so that when `AuthorizationBackend == "kessel"`:
  - Calls `ListAuthorizedResources` for `openshift.cluster` and `openshift.project`.
  - Calls `CheckPermission` for `openshift.node`.
  - Applies fallback: empty list + Check true → `["*"]`; empty list + Check false → `[]`.
  - Sets `user.permissions` with the same shape as RBAC (slice of strings).

- [x] **Error handling**
  On Kessel unavailability: returns **403** (per design §9 Risks and test plan §1.9). No change to handlers or `AddRBACFilter`.

### 6. Query layer (done)

- [x] **No changes**
  `AddRBACFilter` already supports `["*"]` and specific ID lists. It receives the same types from the new middleware (`[]string`).

### 7. Tests (done)

- [x] **Unit tests**
  Mock `PermissionChecker` with both `CheckPermission` and `ListAuthorizedResources`. Tests cover: specific IDs returned; empty + Check true → `["*"]`; empty + Check false → `[]`; 403 on error.

- [x] **Integration tests**
  Run against real Kessel stack (SpiceDB + Relations API). Seed data via Kessel Relations API `CreateTuples`. Verify LookupResources returns expected IDs and Check fallback works.

- [x] **Contract tests**
  Seed via Kessel Relations API `CreateTuples` (not authzed-go). Schema pre-loaded by test harness (`SPICEDB_SCHEMA_FILE` volume mount). Contract tests cover Check and LookupResources.

### 8. Parity checks (done)

- [x] **RBAC vs Kessel**
  For the same user/org, with equivalent roles/bindings:
  - RBAC path and Kessel path produce the same `user.permissions` (same keys, same `["*"]` vs list of IDs).
  - Handlers and `AddRBACFilter` behave identically (same filtered result sets).
  - 403 on auth failure in both paths.

- [x] **Docs**
  [kessel-integration.md](./kessel-integration.md) updated to reflect LookupResources + Check.

---

## Permission / relation names

Aligned with ZED schema:

| ROS resource type     | Kessel resource type                          | LookupResources relation | Check relation (tenant-level) |
|-----------------------|-----------------------------------------------|--------------------------|-------------------------------|
| `openshift.cluster`   | `cost_management/openshift_cluster`           | `read`                   | `cost_management_openshift_cluster_read` |
| `openshift.node`      | (Check only -- no LookupResources)            | N/A                      | `cost_management_openshift_node_read` |
| `openshift.project`   | `cost_management/openshift_project`           | `read`                   | `cost_management_openshift_project_read` |

The ZED schema defines `permission read` on each `cost_management/*` resource type, which resolves through `t_workspace` to the tenant-level permission.

---

## RBAC vs Kessel: how ros-ocp-backend gets permissions

**RBAC path:** ros-ocp-backend calls the **RBAC HTTP API** (`GET /api/rbac/v1/access/?application=cost-management`) and gets a list of ACLs. `aggregate_permissions()` in [`internal/api/middleware/rbac.go`](../../internal/api/middleware/rbac.go) builds the map:

- For each permission (e.g. `cost-management:openshift.cluster:read`), if **ResourceDefinitions** is empty → append `"*"` (wildcard).
- If ResourceDefinitions is present → extract resource IDs from `AttributeFilter.Value` (cluster UUIDs, namespace names) and append them.

So RBAC already produces **either** `["*"]` **or** a list of specific IDs — the same shape Kessel produces. Kessel uses **LookupResources** (for specific IDs) plus **Check** fallback (for wildcard when LookupResources returns empty).

---

## Kessel API only (enforced)

All runtime and test code uses **only the Kessel API**. No direct SpiceDB (`authzed-go` or otherwise) for seeding, schema, or checks.

| Area | Approach |
|--------|----------|
| **Auth checks** | Kessel Relations API: `Check` and `LookupResources`. |
| **Contract test seeding** | Kessel Relations API `KesselTupleService.CreateTuples` (gRPC). Uses `internal/testutil/kessel_seeder.go`. |
| **Schema for tests** | Pre-loaded by the test environment via `SPICEDB_SCHEMA_FILE` volume mount in Docker Compose. Tests do not write schema. |
| **authzed-go** | **Removed** from `go.mod`. Not used anywhere in production or test code. |

---

## Summary

1. **Kessel API only:** No direct SpiceDB in production or tests. Auth via Relations API (`Check` and `LookupResources`); test seeding via Kessel Relations API `CreateTuples`. `authzed-go` removed.
2. Relations API provides both `KesselCheckServiceClient` and `KesselLookupServiceClient` from a single gRPC connection.
3. `PermissionChecker` interface has `CheckPermission` and `ListAuthorizedResources`.
4. `KesselMiddleware` uses `ListAuthorizedResources` for cluster/project and `CheckPermission` for node; preserves `user.permissions` shape.
5. No handler or `AddRBACFilter` changes; 403 on failure.
6. Tests (UT, IT, CT) validate parity with RBAC.
