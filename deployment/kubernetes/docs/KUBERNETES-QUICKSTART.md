# ROS-OCP Kubernetes Quick Start Guide

This guide walks you through deploying and testing the ROS-OCP backend services on a KIND cluster using the Helm chart.

## Prerequisites

### System Resources
Ensure your system has adequate resources for the deployment:
- **Memory**: At least 8GB RAM (12GB+ recommended)
- **CPU**: 4+ cores
- **Storage**: 10GB+ free disk space

The deployment includes:
- 3 PostgreSQL databases (256Mi each)
- Kafka + Zookeeper (512Mi + 256Mi)
- Kruize optimization engine (1-2Gi - most memory intensive)
- Various application services (256-512Mi each)

### Required Tools
Install these tools on your system (macOS commands shown):

```bash
# Install KIND for local Kubernetes clusters
brew install kind

# Install kubectl for Kubernetes management
brew install kubectl

# Install Helm for package management
brew install helm

# Install Podman for container operations
brew install podman
```

### Verify Installation
```bash
kind --version
kubectl version --client
helm version
podman --version
```

## Quick Deployment

### 1. Navigate to Scripts Directory
```bash
cd /path/to/ros-ocp-backend/scripts/
```

### 2. Deploy to KIND Cluster
```bash
# This will create a KIND cluster and deploy all services
./deploy-kind.sh
```

The script will:
- ✅ Check prerequisites
- ✅ Create KIND cluster with proper networking
- ✅ Install storage provisioner
- ✅ Deploy Helm chart with all services
- ✅ Create NodePort services for external access
- ✅ Run health checks

**Expected Output:**
```
[INFO] Running health checks...
[SUCCESS] Ingress API is accessible
[SUCCESS] ROS-OCP API is accessible
[SUCCESS] Kruize API is accessible
[SUCCESS] MinIO console is accessible
[SUCCESS] All health checks passed!
```

### 3. Verify Deployment
```bash
# Check deployment status
./deploy-kind.sh status

# Run health checks
./deploy-kind.sh health
```

## Access Points

After successful deployment, these services are available:

| Service | URL | Description |
|---------|-----|-------------|
| **Ingress API** | http://localhost:30080 | File upload endpoint |
| **ROS-OCP API** | http://localhost:30081 | Main REST API |
| **Kruize API** | http://localhost:30090 | Optimization engine |
| **MinIO Console** | http://localhost:30099 | Storage admin UI |

### Quick Access Test
```bash
# Test Ingress API
curl http://localhost:30080/api/ingress/v1/version

# Test ROS-OCP API
curl http://localhost:30081/status

# Test Kruize API
curl http://localhost:30090/listPerformanceProfiles
```

## End-to-End Data Flow Testing

### 1. Run Complete Test
```bash
# This tests the full data pipeline
./test-k8s-dataflow.sh
```

The test will:
- ✅ Upload test CSV data via Ingress API
- ✅ Simulate Koku service processing
- ✅ Copy data to MinIO ros-data bucket
- ✅ Publish Kafka event for processor
- ✅ Verify data processing and database storage
- ✅ Check Kruize experiment creation

**Expected Output:**
```
[INFO] ROS-OCP Kubernetes Data Flow Test
==================================
[SUCCESS] Step 1: Upload completed successfully
[SUCCESS] Steps 2-3: Koku simulation and Kafka event completed successfully
[SUCCESS] Found 1 workload records in database
[SUCCESS] All health checks passed!
[SUCCESS] Data flow test completed!
```

### 2. View Service Logs
```bash
# List available services
./test-k8s-dataflow.sh logs

# View specific service logs
./test-k8s-dataflow.sh logs rosocp-processor
./test-k8s-dataflow.sh logs ingress
./test-k8s-dataflow.sh logs kruize
```

### 3. Monitor Processing
```bash
# Watch pods in real-time
kubectl get pods -n ros-ocp -w

# Check persistent volumes
kubectl get pvc -n ros-ocp

# View all services
kubectl get svc -n ros-ocp
```

## Manual Testing

### Upload Test File
```bash
# Create test CSV file
cat > test-data.csv << 'EOF'
report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
2024-01-01,2024-01-01,2024-01-01 00:00:00 -0000 UTC,2024-01-01 00:15:00 -0000 UTC,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,100,100,200,200,50,10,90,50,0,0,0,512,512,1024,1024,256,128,384,256,200,100,300,200
EOF

# Important: For Kruize compatibility, ensure:
# - report_period_start and report_period_end should match for short intervals
# - Use timezone format '-0000 UTC' instead of 'Z' for Go time parsing compatibility
# - Keep interval duration under 30 minutes for optimal Kruize validation

# Upload via Ingress API
curl -X POST \
  -F "file=@test-data.csv" \
  -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K" \
  -H "x-rh-request-id: manual-test-$(date +%s)" \
  http://localhost:30080/api/ingress/v1/upload
```

### Check Database
```bash
# Connect to ROS database
kubectl exec -it -n ros-ocp deployment/ros-ocp-db-ros -- \
  psql -U postgres -d postgres -c "SELECT COUNT(*) FROM workloads;"
```

### Monitor Kafka Topics
```bash
# List topics
kubectl exec -n ros-ocp deployment/ros-ocp-kafka -- \
  kafka-topics --list --bootstrap-server localhost:29092

# Monitor events
kubectl exec -n ros-ocp deployment/ros-ocp-kafka -- \
  kafka-console-consumer --bootstrap-server localhost:29092 \
  --topic hccm.ros.events --from-beginning
```

## Troubleshooting

### Common Issues

**Pods getting OOMKilled (Out of Memory):**
```bash
# Check pod status for OOMKilled
kubectl get pods -n ros-ocp

# If you see OOMKilled status, increase memory limits
# Create custom values file
cat > low-resource-values.yaml << EOF
resources:
  kruize:
    requests:
      memory: "512Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "500m"

  database:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "250m"

  application:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "200m"
EOF

# Upgrade with reduced resources
helm upgrade ros-ocp ./ros-ocp-helm -n ros-ocp -f low-resource-values.yaml
```

**Kruize listExperiments API error:**

The Kruize `/listExperiments` endpoint may show errors related to missing `KruizeLMExperimentEntry` entity. This is a known issue with the current Kruize image version, but experiments are still being created and processed correctly in the database.

```bash
# Workaround: Check experiments directly in database
kubectl exec -n ros-ocp ros-ocp-db-kruize-0 -- \
  psql -U postgres -d postgres -c "SELECT experiment_name, status FROM kruize_experiments;"
```

**Kafka connectivity issues (Connection refused errors):**

This is a common issue affecting multiple services (processor, recommendation-poller, housekeeper).

```bash
# Step 1: Check current Kafka status
kubectl get pods -n ros-ocp -l app.kubernetes.io/name=kafka
kubectl logs -n ros-ocp -l app.kubernetes.io/name=kafka --tail=20

# Step 2: Apply the Kafka networking fix and restart
helm upgrade ros-ocp ./ros-ocp-helm -n ros-ocp
kubectl rollout restart statefulset/ros-ocp-kafka -n ros-ocp

# Step 3: Wait for Kafka to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=kafka -n ros-ocp --timeout=300s

# Step 4: Restart all dependent services
kubectl rollout restart deployment/ros-ocp-rosocp-processor -n ros-ocp
kubectl rollout restart deployment/ros-ocp-rosocp-recommendation-poller -n ros-ocp
kubectl rollout restart deployment/ros-ocp-rosocp-housekeeper -n ros-ocp
kubectl rollout restart deployment/ros-ocp-ingress -n ros-ocp

# Step 5: Verify connectivity
kubectl logs -n ros-ocp -l app.kubernetes.io/name=rosocp-processor --tail=10
kubectl exec -n ros-ocp deployment/ros-ocp-rosocp-processor -- nc -zv ros-ocp-kafka 29092
```

**Alternative: Complete redeployment if issues persist:**
```bash
# Delete and redeploy if Kafka issues persist
helm uninstall ros-ocp -n ros-ocp
kubectl delete namespace ros-ocp
./deploy-kind.sh
```

**Pods not starting:**
```bash
# Check pod status and events
kubectl get pods -n ros-ocp
kubectl describe pod -n ros-ocp <pod-name>

# Check logs
kubectl logs -n ros-ocp <pod-name>
```

**Services not accessible:**
```bash
# Check if NodePort services are created
kubectl get svc -n ros-ocp

# Test port forwarding as alternative
kubectl port-forward -n ros-ocp svc/ros-ocp-ingress 3000:3000
kubectl port-forward -n ros-ocp svc/ros-ocp-rosocp-api 8001:8000
```

**Storage issues:**
```bash
# Check persistent volume claims
kubectl get pvc -n ros-ocp

# Check storage class
kubectl get storageclass
```

### Debug Commands

```bash
# Get all resources in namespace
kubectl get all -n ros-ocp

# Check Helm release status
helm status ros-ocp -n ros-ocp

# View Helm values
helm get values ros-ocp -n ros-ocp

# Check cluster info
kubectl cluster-info
```

## Configuration

### Environment Variables
```bash
# Customize deployment
export KIND_CLUSTER_NAME=my-ros-cluster
export HELM_RELEASE_NAME=my-ros-ocp
export NAMESPACE=my-namespace

# Deploy with custom settings
./deploy-kind.sh
```

### Helm Values Override
```bash
# Create custom values file
cat > my-values.yaml << EOF
global:
  storageClass: "fast-ssd"

database:
  ros:
    storage:
      size: 20Gi

resources:
  application:
    requests:
      memory: "256Mi"
      cpu: "200m"
EOF

# Deploy with custom values
helm upgrade --install ros-ocp ./ros-ocp-helm \
  --namespace ros-ocp \
  --create-namespace \
  -f my-values.yaml
```

## Cleanup

### Remove Deployment Only
```bash
# Remove Helm release and namespace
./deploy-kind.sh cleanup
```

### Remove Everything
```bash
# Delete entire KIND cluster
./deploy-kind.sh cleanup --all
```

### Manual Cleanup
```bash
# Delete Helm release
helm uninstall ros-ocp -n ros-ocp

# Delete namespace
kubectl delete namespace ros-ocp

# Delete KIND cluster
kind delete cluster --name ros-ocp-cluster
```

## Quick Status Check

Use this script to verify all services are working:

```bash
#!/bin/bash
echo "=== ROS-OCP Status Check ==="

# Check pod status
echo "Pod Status:"
kubectl get pods -n ros-ocp

# Check services with issues
echo -e "\nPods with issues:"
kubectl get pods -n ros-ocp --field-selector=status.phase!=Running

# Check Kafka connectivity
echo -e "\nKafka connectivity test:"
kubectl exec -n ros-ocp deployment/ros-ocp-rosocp-processor -- nc -zv ros-ocp-kafka 29092 2>/dev/null && echo "✓ Kafka accessible" || echo "✗ Kafka connection failed"

# Check API endpoints
echo -e "\nAPI Health Checks:"
curl -s http://localhost:30080/api/ingress/v1/version >/dev/null && echo "✓ Ingress API" || echo "✗ Ingress API failed"
curl -s http://localhost:30081/status >/dev/null && echo "✓ ROS-OCP API" || echo "✗ ROS-OCP API failed"
curl -s http://localhost:30090/listPerformanceProfiles >/dev/null && echo "✓ Kruize API" || echo "✗ Kruize API failed"

echo -e "\nFor detailed troubleshooting, run: ./test-k8s-dataflow.sh health"
```

## Next Steps

After successful deployment:

1. **Explore APIs**: Use the access points to interact with services
2. **Load Test Data**: Upload your own cost management files
3. **Monitor Metrics**: Check Kruize recommendations and optimizations
4. **Scale Services**: Modify Helm values to scale deployments
5. **Production Setup**: Adapt for real Kubernetes clusters

## Support

For issues or questions:
- Check pod logs: `kubectl logs -n ros-ocp <pod-name>`
- Review test output: `./test-k8s-dataflow.sh`
- Verify configuration: `helm get values ros-ocp -n ros-ocp`