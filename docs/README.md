# ROS-OCP Backend Documentation

This directory contains comprehensive documentation for the ROS-OCP backend project.

## Quick Start

### Docker Compose Deployment
```bash
# Navigate to the deployment directory
cd ros-ocp-backend/deployment/docker-compose/

# Set environment variables (optional, defaults will be used)
export INGRESS_PORT=3000
export MINIO_ACCESS_KEY=minioaccesskey
export MINIO_SECRET_KEY=miniosecretkey

# Start services and run complete data flow test
./test-ros-ocp-dataflow.sh
```

### Kubernetes Deployment
```bash
# Navigate to the kubernetes scripts directory
cd ros-ocp-backend/deployment/kubernetes/scripts/

# Run the complete data flow test (assumes deployment is running)
./test-k8s-dataflow.sh
```

## Documentation Files

- `README.md` - This documentation overview file
- `ROS-OCP-DATAFLOW.md` - Detailed data flow architecture documentation

## Project Structure

For complete project organization details, see `../DIRECTORY-STRUCTURE.md`

### Key Testing Locations
- **Docker Compose Testing**: `../deployment/docker-compose/test-ros-ocp-dataflow.sh`
- **Kubernetes Testing**: `../deployment/kubernetes/scripts/test-k8s-dataflow.sh`
- **Sample Data**: `../testing/samples/` and `../deployment/docker-compose/samples/`

## Docker Compose Test Script Features

The `../deployment/docker-compose/test-ros-ocp-dataflow.sh` script provides:

### 🚀 Service Management
- Starts all services using podman-compose with proper dependencies
- Waits for services to be healthy before proceeding
- Shows service status and logs
- Provides cleanup functionality

### 📊 Complete Data Flow Testing
1. **Upload Phase**: Tests file upload via Ingress API
2. **Storage Phase**: Verifies files are stored in MinIO bucket
3. **Messaging Phase**: Checks Kafka events are produced
4. **Processing Phase**: Confirms data is processed and stored in database

### 🔍 Verification Steps
- **MinIO Bucket**: Checks if uploaded files are stored
- **Kafka Topics**: Monitors `hccm.ros.events` and `rosocp.kruize.recommendations` topics
- **Database**: Verifies workload data is inserted into PostgreSQL
- **Service Health**: Confirms all services are running and responsive

## Usage Examples

### Docker Compose Usage Examples

```bash
# Navigate to deployment directory
cd ../deployment/docker-compose/

# Run complete test
./test-ros-ocp-dataflow.sh

# Check service status
./test-ros-ocp-dataflow.sh status

# View logs
./test-ros-ocp-dataflow.sh logs
./test-ros-ocp-dataflow.sh logs rosocp-processor

# Clean up
./test-ros-ocp-dataflow.sh cleanup
```

### Kubernetes Usage Examples

```bash
# Navigate to kubernetes scripts directory
cd ../deployment/kubernetes/scripts/

# Run complete test
./test-k8s-dataflow.sh

# Run health checks only
./test-k8s-dataflow.sh health

# View service logs
./test-k8s-dataflow.sh logs rosocp-processor
```

## Services Started

The script starts the following services:

### Core Infrastructure
- **PostgreSQL** (3 instances): ROS, Kruize, and Sources databases
- **Kafka + Zookeeper**: Message streaming
- **Redis**: Caching
- **MinIO**: Object storage with bucket setup

### Application Services
- **Ingress**: File upload API (`localhost:3000`)
- **ROS-OCP API**: Main API service (`localhost:8001`)
- **ROS-OCP Processor**: Processes uploaded data
- **ROS-OCP Recommendation Poller**: Polls Kruize for recommendations
- **ROS-OCP Housekeeper**: Cleanup and maintenance
- **Kruize Autotune**: Recommendation engine (`localhost:8080`)
- **Sources API**: Sources management (`localhost:8002`)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INGRESS_PORT` | `3000` | Port for Ingress API |
| `MINIO_ACCESS_KEY` | `minioaccesskey` | MinIO access key |
| `MINIO_SECRET_KEY` | `miniosecretkey` | MinIO secret key |

## Test Data

The scripts use test data from:
- `../deployment/docker-compose/samples/cost-mgmt.tar.gz` - Docker Compose test file
- `../testing/samples/cost-mgmt.tar.gz` - Kubernetes test file

## Data Flow Verification

The script verifies the complete data flow:

```
Upload File → Ingress API → MinIO Storage → Kafka Event →
ROS Processor → Kruize API → Database Storage
```

### Step-by-Step Verification:

1. **File Upload**: POST to `/api/ingress/v1/upload` with tar.gz file
2. **MinIO Check**: Verifies file is stored in `insights-upload-perma` bucket
3. **Kafka Check**: Confirms events in `hccm.ros.events` topic
4. **Processing Check**: Monitors `rosocp.kruize.recommendations` topic
5. **Database Check**: Verifies workload records in PostgreSQL

## Troubleshooting

### Common Issues

**Services not starting**:
```bash
# For Docker Compose
cd ../deployment/docker-compose/
./test-ros-ocp-dataflow.sh logs [service-name]
podman-compose ps

# For Kubernetes
cd ../deployment/kubernetes/scripts/
./test-k8s-dataflow.sh logs [service-name]
kubectl get pods -n ros-ocp
```

**Upload failures**:
- Ensure Ingress service is running: `curl http://localhost:3000/api/ingress/v1/version`
- Check MinIO is accessible: `curl http://localhost:9000/minio/health/live`
- Verify Kafka is ready: `podman exec kafka_1 kafka-topics --list --bootstrap-server localhost:29092` (Docker Compose)

**No data in database**:
- Check processor logs: Use respective test script `logs rosocp-processor` command
- Verify Kruize is responding: `curl http://localhost:8080/listPerformanceProfiles` (Docker Compose) or `curl http://localhost:30090/listPerformanceProfiles` (Kubernetes)
- Check database connection: Use `podman exec db-ros_1 pg_isready -U postgres` (Docker Compose) or `kubectl exec -n ros-ocp <db-pod> -- pg_isready -U postgres` (Kubernetes)

### Manual Testing

**Upload test file manually (Docker Compose)**:
```bash
cd ../deployment/docker-compose/
curl -F "upload=@samples/cost-mgmt.tar.gz;type=application/vnd.redhat.hccm.tar+tgz" \
     -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0=" \
     http://localhost:3000/api/ingress/v1/upload
```

**Check MinIO bucket (Docker Compose)**:
```bash
podman exec minio_1 /usr/bin/mc ls myminio/insights-upload-perma/
```

**Monitor Kafka topics (Docker Compose)**:
```bash
podman exec kafka_1 kafka-console-consumer \
    --bootstrap-server localhost:29092 \
    --topic hccm.ros.events \
    --from-beginning
```

**Query database (Docker Compose)**:
```bash
podman exec db-ros_1 psql -U postgres -d postgres -c "SELECT COUNT(*) FROM workloads;"
```

## Access Points

### Docker Compose Endpoints
- **Ingress API**: http://localhost:3000/api/ingress/v1/version
- **ROS-OCP API**: http://localhost:8001/status
- **Kruize API**: http://localhost:8080/listPerformanceProfiles
- **MinIO Console**: http://localhost:9990 (admin UI)
- **Sources API**: http://localhost:8002/api/sources/v1.0/source_types

### Kubernetes Endpoints (default ports)
- **Ingress API**: http://localhost:30080/api/ingress/v1/version
- **ROS-OCP API**: http://localhost:30081/status
- **Kruize API**: http://localhost:30090/listPerformanceProfiles
- **MinIO Console**: http://localhost:30099 (admin UI)

## Notes

- Both deployment methods use podman-compose/kubectl as specified in the project's CLAUDE.md guidelines
- All services are configured to use insights-onprem images where available
- MinIO bucket is automatically created and configured for public access
- Scripts include comprehensive error handling and colored output for better readability
- Services remain running after tests complete for further manual testing
- For detailed deployment instructions, see the respective README files in `../deployment/` directories