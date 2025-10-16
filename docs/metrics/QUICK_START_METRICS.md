# Quick Start: Disabling Metrics for Integration Tests

## TL;DR

To disable the metrics endpoint and avoid Kubernetes dependency:

```bash
export METRICS_ENABLED=false
./rosocp start
```

## Integration Test Example

```bash
#!/bin/bash
# integration-test.sh

# Disable metrics to avoid K8s dependency
export METRICS_ENABLED=false

# Start services
docker-compose up -d db-ros

# Start ROS-OCP backend
./rosocp start &
ROSOCP_PID=$!

# Wait for server to start
sleep 2

# Run integration tests against REST API
# Metrics endpoint will not be running (no K8s required)
curl -H "X-Rh-Identity: $(echo -n '{"identity":{"org_id":"test"}}' | base64)" \
  http://localhost:8000/api/cost-management/v1/recommendations/openshift

# Cleanup
kill $ROSOCP_PID
```

## Configuration Options

| Scenario | METRICS_ENABLED | Result |
|----------|----------------|--------|
| **Production** | `true` (default) | Metrics on port 9000 with OAuth2 |
| **Integration Tests** | `false` | Metrics disabled, no K8s needed |
| **Local Dev with K8s** | `true` | Metrics on port 5005 with OAuth2 |

## What Changed?

### Before (Original Code)
```go
// Metrics endpoint had no authentication
metrics.GET("/metrics", echoprometheus.NewHandler())
```

### After (New Code)
```go
// 1. Metrics endpoint now has OAuth2 authentication
// 2. Can be disabled via METRICS_ENABLED=false

if cfg.MetricsEnabled {
    oauth2Handler, _ := ros_middleware.GetIdentityProviderHandlerFunction(
        ros_middleware.OAuth2IDProvider)
    metrics.Use(oauth2Handler)
    metrics.GET("/metrics", echoprometheus.NewHandler())
} else {
    log.Info("Metrics endpoint disabled")
}
```

### REST API Endpoints (Unchanged)
```go
// REST API always uses RHSSO authentication
rhssoHandler, _ := ros_middleware.GetIdentityProviderHandlerFunction(
    ros_middleware.RHSSOIDProvider)
v1.Use(rhssoHandler)
v1.GET("/recommendations/openshift", GetRecommendationSetList)
```

## See Also

- [METRICS_CONFIG.md](./METRICS_CONFIG.md) - Complete documentation
- [clowdapp.yaml](./clowdapp.yaml) - Health probe configuration

