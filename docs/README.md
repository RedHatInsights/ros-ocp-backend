# ROS-OCP Backend Documentation

This directory contains comprehensive documentation for the ROS-OCP backend project.

## Testing Workflows Overview

The ROS-OCP backend provides **dual testing workflows** for Kubernetes deployments:

| Workflow | Purpose | Scripts | Environment |
|----------|---------|---------|-------------|
| **CI/CD Validation** | Automated testing & continuous integration | `deploy-kind.sh` + `install-helm-chart.sh` + `test-k8s-dataflow.sh` | KIND (ephemeral) |
| **OpenShift Production** | Production deployment validation | `install-helm-chart.sh` + `test-ocp-dataflow.sh` | OpenShift clusters |
| **Universal Deployment** | Cross-platform installation with validation | `install-helm-chart.sh` (includes built-in validation) | Both K8s & OpenShift |

### Key Benefits
- **CI/CD**: Disposable KIND clusters for reliable automated testing with comprehensive deployment validation
- **OpenShift**: Platform-specific validation with Routes and dynamic cluster detection
- **Universal**: Single deployment script with automatic platform detection and built-in validation

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

The Kubernetes deployment provides two distinct testing workflows:

#### 1. CI/CD Validation (Vanilla Kubernetes)
For automated testing and continuous integration using KIND:
```bash
# Navigate to the kubernetes scripts directory
cd ros-ocp-backend/deployment/kubernetes/scripts/

# Step 1: Setup KIND cluster
./deploy-kind.sh

# Step 2: Deploy ROS-OCP Helm chart with built-in validation
./install-helm-chart.sh

# Step 3: Run CI/CD validation tests
./test-k8s-dataflow.sh
```

**Note**: The `install-helm-chart.sh` script includes comprehensive deployment validation:
- Prerequisites verification (kubectl, helm, jq)
- Platform detection (Kubernetes vs OpenShift)
- Kafka cluster conflict detection and resolution
- Post-deployment health checks for all services
- Service connectivity validation

#### 2. OpenShift Production Deployment
For OpenShift clusters using automatic platform detection:
```bash
# Navigate to the kubernetes scripts directory
cd ros-ocp-backend/deployment/kubernetes/scripts/

# Deploy to OpenShift (auto-detects platform)
./install-helm-chart.sh

# Run OpenShift-specific validation
./test-ocp-dataflow.sh
```

## Documentation Files

- `README.md` - This documentation overview file
- `ROS-OCP-DATAFLOW.md` - Detailed data flow architecture documentation

## Project Structure

For complete project organization details, see `../DIRECTORY-STRUCTURE.md`

### Key Testing Locations
- **Docker Compose Testing**: `../deployment/docker-compose/test-ros-ocp-dataflow.sh`
- **Kubernetes CI/CD Testing**: `../deployment/kubernetes/scripts/test-k8s-dataflow.sh` (KIND-based validation)
- **OpenShift Testing**: `../deployment/kubernetes/scripts/test-ocp-dataflow.sh` (OpenShift-specific validation)
- **Universal Deployment with Validation**: `../deployment/kubernetes/scripts/install-helm-chart.sh` (both K8s and OpenShift with built-in health checks)
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

## Testing Workflows

### 1. CI/CD Validation Workflow (Vanilla Kubernetes)
**Purpose**: Automated testing and continuous integration validation using KIND clusters.

```bash
# Navigate to kubernetes scripts directory
cd ../deployment/kubernetes/scripts/

# Setup ephemeral KIND cluster for testing
./deploy-kind.sh

# Deploy Helm chart with comprehensive validation (auto-detects Kubernetes platform)
./install-helm-chart.sh

# Run comprehensive CI/CD validation tests
./test-k8s-dataflow.sh

# Run additional health checks
./install-helm-chart.sh health

# View service logs during CI/CD
./test-k8s-dataflow.sh logs rosocp-processor

# Cleanup (delete Helm release only)
./install-helm-chart.sh cleanup

# Full CI/CD cleanup (delete entire KIND cluster)
./deploy-kind.sh cleanup --all
```

**CI/CD Validation Features**:
- **Deployment Validation**: `install-helm-chart.sh` performs automated deployment verification
- **Prerequisites Check**: Validates kubectl, helm, jq availability and cluster connectivity
- **Platform Detection**: Automatically detects and configures for Kubernetes vs OpenShift
- **Service Health Validation**: Built-in health checks for all deployed services
- **Conflict Resolution**: Automatic detection and resolution of Kafka cluster ID conflicts
- **Post-deployment Verification**: Ensures all pods are ready and services are accessible

### 2. OpenShift Production Deployment Workflow
**Purpose**: Production deployment and validation for OpenShift clusters.

```bash
# Navigate to kubernetes scripts directory
cd ../deployment/kubernetes/scripts/

# Deploy to OpenShift (auto-detects platform and uses OpenShift-specific values)
./install-helm-chart.sh

# Run OpenShift-specific validation tests
./test-ocp-dataflow.sh

# OpenShift health checks
./install-helm-chart.sh health

# OpenShift-specific debugging
./test-ocp-dataflow.sh logs rosocp-processor

# OpenShift cleanup (preserves persistent data)
./install-helm-chart.sh cleanup

# Complete OpenShift cleanup (removes all data)
./install-helm-chart.sh cleanup --complete
```

### 3. Universal Deployment Script
**Purpose**: `install-helm-chart.sh` manages Helm chart installation and validation across both environments.

**Deployment Features**:
- **Automatic Platform Detection**: Detects Kubernetes vs OpenShift
- **Dynamic Configuration**: Uses appropriate values files (values.yaml vs openshift-values.yaml)
- **Intelligent Storage**: Auto-detects optimal storage classes
- **Conflict Resolution**: Handles Kafka cluster ID conflicts automatically

**CI/CD Validation Features**:
- **Prerequisites Validation**: Verifies kubectl, helm, jq installation and cluster connectivity
- **Deployment Health Checks**: Validates all services are running and accessible
- **Service Connectivity Tests**: Tests internal and external service endpoints
- **Kafka Topic Verification**: Ensures required Kafka topics are created
- **Platform-Specific Validation**: Adapts health checks for Kubernetes vs OpenShift environments

## Deployment Validation Details

The `install-helm-chart.sh` script provides comprehensive CI/CD validation capabilities:

### Pre-deployment Validation
- **Tool Prerequisites**: Verifies kubectl, helm, and jq are installed and accessible
- **Cluster Connectivity**: Tests kubectl connection to target cluster and validates current context
- **Platform Detection**: Automatically detects Kubernetes vs OpenShift environments
- **Conflict Detection**: Identifies and resolves Kafka cluster ID conflicts before deployment

### During Deployment
- **Helm Chart Validation**: Uses Helm's built-in validation with `--wait` flag
- **Resource Creation**: Monitors namespace creation and resource allocation
- **Pod Readiness**: Waits for all pods to reach ready state (900s timeout for full deployment)
- **Kafka Topic Creation**: Automatically creates required topics: `hccm.ros.events`, `rosocp.kruize.recommendations`, `platform.upload.announce`, `platform.payload-status`

### Post-deployment Health Checks
The script includes a comprehensive health check function (`run_health_checks()`) that validates:

#### For OpenShift Platforms:
- **Internal Service Connectivity**: Tests service endpoints within the cluster
- **External Route Accessibility**: Validates OpenShift routes when accessible
- **Service-specific Health**: Tests ROS-OCP API (`/status`), Ingress API (`/api/ingress/v1/version`), and Kruize API (`/listPerformanceProfiles`)

#### For Kubernetes/KIND Platforms:
- **Ingress Connectivity**: Validates access via ingress at `http://localhost/api/ingress/v1/version`
- **API Endpoints**: Tests ROS-OCP API at `http://localhost/status`
- **Kruize Integration**: Validates Kruize API at `http://localhost/api/kruize/listPerformanceProfiles`

### Validation Commands
The script provides additional validation commands for CI/CD pipelines:
```bash
# Run health checks only
./install-helm-chart.sh health

# Show deployment status
./install-helm-chart.sh status

# Clean validation (preserves data)
./install-helm-chart.sh cleanup

# Complete validation cleanup
./install-helm-chart.sh cleanup --complete
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