# Network Policy Security for Metrics Endpoints

## Overview

The ros-ocp-backend uses **Kubernetes NetworkPolicy** to secure metrics endpoints instead of application-level OAuth2 authentication. This approach provides:

- ✅ **Simpler architecture** - No authentication logic in application code
- ✅ **Better security** - Network isolation is enforced at the CNI layer
- ✅ **Improved testability** - No authentication complexity in integration tests
- ✅ **Kubernetes-native** - Leverages platform capabilities

## Architecture

### Security Model

```
┌──────────────────────────┐
│ openshift-monitoring     │
│ (Prometheus)             │
└────────────┬─────────────┘
             │ Port 9000
             │ ✅ ALLOWED
             ▼
┌──────────────────────────┐
│ ros-ocp-backend          │
│ /metrics endpoint        │
│ (NetworkPolicy enforced) │
└──────────────────────────┘
             ▲
             │ Port 9000
             │ ❌ DENIED
┌────────────┴─────────────┐
│ Other namespaces/pods    │
└──────────────────────────┘
```

### Endpoints Secured

| Endpoint | Port | Security Method | Access |
|----------|------|----------------|--------|
| `/metrics` | 9000 | NetworkPolicy | openshift-monitoring only |
| `/api/cost-management/v1/recommendations/openshift` | 8000 | RHSSO (X-Rh-Identity) | Via ingress with auth |
| `/status` | 8000 | None (public) | Health checks |

## NetworkPolicy Configuration

### Location
`deployment/kubernetes/networkpolicy-metrics.yaml`

### Policies Defined

1. **`ros-ocp-metrics-access`** - Allows Prometheus to scrape ros-ocp-backend metrics
2. **`ros-ocp-processor-metrics-access`** - Allows Prometheus to scrape processor metrics
3. **`ros-ocp-poller-metrics-access`** - Allows Prometheus to scrape poller metrics
4. **`ros-ocp-default-deny-ingress`** - Default deny all ingress (explicit allow required)
5. **`ros-ocp-api-access`** - Allows ingress/router to access REST API

### Key Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ros-ocp-metrics-access
  namespace: ros-ocp
spec:
  podSelector:
    matchLabels:
      app: ros-ocp-backend
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: openshift-monitoring  # OpenShift monitoring namespace
    ports:
    - protocol: TCP
      port: 9000  # Metrics port
```

## Application Changes

### Removed Components

1. **OAuth2 authentication for metrics** - No longer needed
2. **`METRICS_ENABLED` configuration flag** - Metrics always enabled
3. **Service account token authentication** - Internal services use NetworkPolicy
4. **TokenReview API dependency** - No Kubernetes API calls for auth

### Simplified Code

**Before (OAuth2):**
```go
oauth2Handler, err := ros_middleware.GetIdentityProviderHandlerFunction(ros_middleware.OAuth2IDProvider)
if err != nil {
    log.Fatalf("Failed to initialize OAuth2: %v", err)
}

metrics := echo.New()
metrics.Use(oauth2Handler)  // Complex authentication middleware
metrics.GET("/metrics", echoprometheus.NewHandler())
```

**After (NetworkPolicy):**
```go
metrics := echo.New()
metrics.GET("/metrics", echoprometheus.NewHandler())  // Simple and secure!
log.Infof("Starting metrics endpoint (secured by NetworkPolicy)")
```

## Deployment

### Prerequisites

1. **CNI Plugin Support** - Cluster must have NetworkPolicy support
   - ✅ OpenShift (default)
   - ✅ Calico
   - ✅ Cilium
   - ✅ Weave Net

2. **Monitoring Namespace Labels** - Ensure monitoring namespace has correct labels:
   ```bash
   kubectl label namespace openshift-monitoring name=openshift-monitoring
   ```

### Applying NetworkPolicies

```bash
# Apply to cluster
kubectl apply -f deployment/kubernetes/networkpolicy-metrics.yaml

# Verify policies
kubectl get networkpolicy -n ros-ocp

# Test access (should succeed from monitoring namespace)
kubectl exec -n openshift-monitoring <prometheus-pod> -- curl http://ros-ocp-backend.ros-ocp:9000/metrics

# Test access (should fail from other namespace)
kubectl run test -n default --rm -it --image=curlimages/curl -- curl http://ros-ocp-backend.ros-ocp:9000/metrics
```

## Troubleshooting

### Issue: Prometheus Can't Scrape Metrics

**Symptoms:**
- Prometheus shows target as "down"
- Metrics not appearing in monitoring dashboards

**Resolution:**
1. Check NetworkPolicy is applied:
   ```bash
   kubectl get networkpolicy -n ros-ocp ros-ocp-metrics-access
   ```

2. Verify monitoring namespace labels:
   ```bash
   kubectl get namespace openshift-monitoring --show-labels
   ```

3. Check metrics endpoint is running:
   ```bash
   kubectl logs -n ros-ocp deployment/ros-ocp-backend | grep "Starting metrics endpoint"
   ```

### Issue: CNI Doesn't Support NetworkPolicy

**Symptoms:**
- NetworkPolicy resources create successfully but don't enforce rules
- All traffic allowed regardless of policy

**Resolution:**
- Verify CNI plugin supports NetworkPolicy
- Consider upgrading cluster or CNI plugin
- Alternative: Use service mesh (Istio/Linkerd) for network segmentation

### Issue: Need Metrics in Local Development

**Symptoms:**
- Can't access metrics when running locally (outside Kubernetes)

**Resolution:**
- Local development doesn't use NetworkPolicy
- Metrics endpoint runs without authentication on `localhost:9000`
- Access directly: `curl http://localhost:9000/metrics`

## Security Considerations

### Defense in Depth

NetworkPolicy provides **network-layer security**. Combined with:
- **Application-layer auth** (RHSSO for REST API)
- **RBAC** (Kubernetes permissions)
- **Pod Security Standards** (runtime security)

### Limitations

1. **Namespace-level granularity** - Can't restrict to specific service accounts within namespace
2. **No audit trail** - Can't log who accessed metrics (but Prometheus scrapes are predictable)
3. **Kubernetes-only** - Doesn't work outside Kubernetes (but fine for local dev)

### Best Practices

1. ✅ **Default Deny** - Use `ros-ocp-default-deny-ingress` policy
2. ✅ **Explicit Allow** - Only allow required traffic
3. ✅ **Separate Policies** - One policy per service/component
4. ✅ **Label Selectors** - Use consistent labels for policy enforcement

## Migration Notes

### What Changed

| Before | After |
|--------|-------|
| OAuth2 TokenReview authentication | NetworkPolicy enforcement |
| `METRICS_ENABLED` config flag | Always enabled |
| Service account tokens for internal calls | No authentication needed |
| Complex middleware setup | Simple endpoint registration |

### Backward Compatibility

- ✅ **Metrics format unchanged** - Same Prometheus metrics
- ✅ **Endpoint path unchanged** - Still `/metrics` on port 9000
- ✅ **Scrape configuration unchanged** - Prometheus config stays same
- ✅ **Local dev works** - No authentication locally

## References

- [Kubernetes NetworkPolicy Documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [OpenShift NetworkPolicy Guide](https://docs.openshift.com/container-platform/latest/networking/network_policy/about-network-policy.html)
- [Prometheus Scraping Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)

## Change History

- **2025-01-XX** - Migrated from OAuth2 to NetworkPolicy for metrics security
- Removed `METRICS_ENABLED` configuration flag
- Simplified internal service-to-service HTTP calls
- Added comprehensive NetworkPolicy definitions

