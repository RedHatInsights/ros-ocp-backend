# Testing Scripts

End-to-end testing scripts for both deployment methods.

## Scripts Overview

### `test-k8s-dataflow.sh`
Comprehensive end-to-end testing for Kubernetes deployment.

**Features:**
- Tests complete data flow from upload to database storage
- Verifies Kruize integration and experiment creation
- Includes health checks for all services
- Supports individual service log viewing

**Usage:**
```bash
# Run complete test
./test-k8s-dataflow.sh

# Run health checks only
./test-k8s-dataflow.sh health

# View service logs
./test-k8s-dataflow.sh logs rosocp-processor
./test-k8s-dataflow.sh logs kruize
```

### `test-ros-ocp-dataflow.sh`
Comprehensive end-to-end testing for Docker Compose deployment.

**Features:**
- Tests complete data flow using podman-compose
- Manages service lifecycle (start, test, cleanup)
- Verifies MinIO, Kafka, and database integration
- Comprehensive error handling and logging

**Usage:**
```bash
# Run complete test with service management
./test-ros-ocp-dataflow.sh

# Check service status
./test-ros-ocp-dataflow.sh status

# View logs
./test-ros-ocp-dataflow.sh logs [service-name]

# Cleanup
./test-ros-ocp-dataflow.sh cleanup
```

## Test Data Flow

Both scripts test the complete ROS-OCP data processing pipeline:

```
1. Upload → Ingress API receives tar.gz file
2. Storage → File stored in MinIO bucket  
3. Event → Kafka event published to hccm.ros.events
4. Processing → ROS-OCP processor downloads and processes data
5. Integration → Data sent to Kruize for optimization analysis
6. Storage → Workload data stored in PostgreSQL database
7. Recommendations → Kruize generates optimization recommendations
```

## Test Verification

### Kubernetes (`test-k8s-dataflow.sh`)
- ✅ All pods ready and healthy
- ✅ File uploaded via Ingress API (HTTP 202)
- ✅ CSV file accessible from processor pod
- ✅ Kafka message published successfully  
- ✅ Workload data found in database
- ✅ Kruize experiments created (verified via database)
- ✅ All API endpoints accessible

### Docker Compose (`test-ros-ocp-dataflow.sh`)
- ✅ All services started and healthy
- ✅ File upload via Ingress API
- ✅ File stored in MinIO bucket
- ✅ Kafka events produced and consumed
- ✅ Data processed and stored in database
- ✅ Kruize integration working
- ✅ Services responsive to API calls

## Environment Variables

### Kubernetes Testing
```bash
NAMESPACE=ros-ocp              # Kubernetes namespace
HELM_RELEASE_NAME=ros-ocp      # Helm release name
INGRESS_PORT=30080             # Ingress NodePort
API_PORT=30081                 # API NodePort
KRUIZE_PORT=30090              # Kruize NodePort
```

### Docker Compose Testing
```bash
INGRESS_PORT=3000              # Ingress port
MINIO_ACCESS_KEY=minioaccesskey # MinIO credentials
MINIO_SECRET_KEY=miniosecretkey
```

## Sample Data

Both scripts use test data from the `../samples/` directory:
- Sample CSV files with proper ROS-OCP format
- Test tar.gz archives for upload testing
- Realistic workload data for processing verification

## Expected Output

Successful test runs will show:
- 🔵 **[INFO]** messages for test steps
- 🟢 **[SUCCESS]** messages for completed verifications  
- 🟡 **[WARNING]** messages for non-critical issues
- 🔴 **[ERROR]** messages for failures

Example successful output:
```
[INFO] ROS-OCP Kubernetes Data Flow Test
[INFO] ==================================
[SUCCESS] All pods are ready
[SUCCESS] Step 1: Upload completed successfully
[SUCCESS] Steps 2-3: Koku simulation and Kafka event completed successfully  
[SUCCESS] Found 1 workload records in database
[SUCCESS] Found 1 Kruize experiment(s) in database
[SUCCESS] All health checks passed!
[SUCCESS] Data flow test completed!
```

## Troubleshooting

**Test failures:**
1. Check service logs using the scripts' log commands
2. Verify all prerequisites are met (kubectl, helm, podman)
3. Ensure sufficient system resources
4. Check network connectivity and port availability

**Common issues:**
- **Pod/container startup**: Check resource limits and pull policies
- **Upload failures**: Verify Ingress service accessibility
- **Processing errors**: Check Kruize and database connectivity  
- **Database issues**: Verify PostgreSQL pod readiness

**Manual verification:**
```bash
# Check specific service health
curl http://localhost:30080/api/ingress/v1/version  # Kubernetes
curl http://localhost:3000/api/ingress/v1/version   # Docker Compose
```

## Notes

- Both scripts include comprehensive cleanup on exit
- Tests use realistic data that matches production formats
- Scripts follow the project's guidelines for using Podman over Docker
- Detailed logging helps with troubleshooting failures