# Metrics Endpoint Configuration

## Overview

The ROS-OCP backend provides a Prometheus metrics endpoint that can be conditionally enabled/disabled via configuration. This is useful for integration testing and development environments where Kubernetes/OAuth2 infrastructure may not be available.

## Configuration

### Environment Variable

```bash
METRICS_ENABLED=<true|false>
```

**Default:** `true` (metrics enabled)

### Behavior

#### When `METRICS_ENABLED=true` (default)
- Metrics endpoint starts on port configured by `PROMETHEUS_PORT` (default: 5005)
- Endpoint is protected with **OAuth2 authentication** (Kubernetes TokenReview)
- Requires valid Bearer token from Kubernetes cluster
- RHSSO authentication is **NOT** supported for metrics endpoint
- Metrics endpoint runs on separate HTTP server in goroutine

#### When `METRICS_ENABLED=false`
- Metrics endpoint is **completely disabled**
- No OAuth2/Kubernetes infrastructure required
- Useful for integration tests and local development
- Prometheus middleware still collects metrics (but endpoint doesn't expose them)

## Authentication Split

### Metrics Endpoint (`/metrics`)
- **Port:** Configured via `PROMETHEUS_PORT` (default: 5005 local, 9000 Clowder)
- **Authentication:** OAuth2 (Kubernetes TokenReview API)
- **Can be disabled:** Yes, via `METRICS_ENABLED=false`

### REST API Endpoints (`/api/cost-management/v1/recommendations/openshift`)
- **Port:** Configured via `API_PORT` (default: 8000)
- **Authentication:** RHSSO (X-Rh-Identity header)
- **Can be disabled:** No, always enabled

### Public Endpoints
- **Port:** Same as REST API (8000)
- **Authentication:** None
- **Endpoints:**
  - `/status` - Health check
  - `/api/cost-management/v1/recommendations/openshift/openapi.json` - API spec

## Usage Examples

### Production/Staging (Metrics Enabled)
```bash
# Default behavior - metrics enabled with OAuth2
export PROMETHEUS_PORT=9000
./rosocp start

# Or explicitly enable
export METRICS_ENABLED=true
export PROMETHEUS_PORT=9000
./rosocp start
```

Access metrics:
```bash
curl -H "Authorization: Bearer <k8s-token>" http://localhost:9000/metrics
```

### Integration Tests (Metrics Disabled)
```bash
# Disable metrics to avoid Kubernetes dependency
export METRICS_ENABLED=false
./rosocp start

# Metrics endpoint will not be available
# curl http://localhost:5005/metrics  # Connection refused
```

### Local Development with Metrics
```bash
# Start local Kubernetes cluster (e.g., KIND)
kind create cluster

# Enable metrics with OAuth2
export METRICS_ENABLED=true
export PROMETHEUS_PORT=5005
export KUBECONFIG=~/.kube/config
./rosocp start

# Access metrics with ServiceAccount token
TOKEN=$(kubectl create token default -n default)
curl -H "Authorization: Bearer $TOKEN" http://localhost:5005/metrics
```

### Docker Compose
```yaml
services:
  rosocp-api:
    environment:
      - METRICS_ENABLED=false  # Disable for local testing
      - API_PORT=8000
    ports:
      - "8000:8000"
```

## Clowder Integration

When running in Clowder environments (OpenShift):
- `METRICS_ENABLED` is **always** set to `true` (cannot be overridden)
- `PROMETHEUS_PORT` is set from Clowder config (`c.MetricsPort`)
- OAuth2 authentication uses in-cluster Kubernetes API
- ServiceMonitor automatically configured to scrape metrics

## Health Checks

**Important:** Kubernetes liveness/readiness probes should use `/status` endpoint, **NOT** `/metrics`:

```yaml
livenessProbe:
  httpGet:
    path: /status  # ✅ Correct - no authentication required
    port: 8000

# DON'T use /metrics for health probes:
livenessProbe:
  httpGet:
    path: /metrics  # ❌ Wrong - requires OAuth2 authentication
    port: 9000
```

## Troubleshooting

### Metrics endpoint returns 401 Unauthorized
- **Cause:** Missing or invalid Bearer token
- **Solution:** Ensure valid Kubernetes ServiceAccount token in Authorization header

### Metrics endpoint connection refused
- **Possible causes:**
  1. `METRICS_ENABLED=false` - Check environment variable
  2. Port already in use - Check `PROMETHEUS_PORT` configuration
  3. OAuth2 initialization failed - Check logs for Kubernetes client errors

### Integration tests fail with Kubernetes errors
- **Cause:** Tests trying to access metrics endpoint without K8s cluster
- **Solution:** Set `METRICS_ENABLED=false` for integration tests

## Configuration Summary

| Variable | Default | Clowder | Description |
|----------|---------|---------|-------------|
| `METRICS_ENABLED` | `true` | `true` (forced) | Enable/disable metrics endpoint |
| `PROMETHEUS_PORT` | `5005` | From Clowder | Port for metrics endpoint |
| `API_PORT` | `8000` | From Clowder | Port for REST API |
| `ID_PROVIDER` | `rhsso` | From Clowder | Authentication for REST API (not metrics) |

## Migration Notes

### From Previous Versions

If you were using:
```bash
ID_PROVIDER=oauth2  # Old behavior: OAuth2 for all endpoints
```

Update to:
```bash
# Metrics now use OAuth2 by default, REST API uses RHSSO
# No changes needed - this is the new default behavior
```

The `ID_PROVIDER` environment variable now only affects REST API endpoints. Metrics endpoint always uses OAuth2 (when enabled).

