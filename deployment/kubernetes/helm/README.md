# ROS-OCP Helm Chart

Kubernetes Helm chart for deploying the complete ROS-OCP backend stack.

## Quick Start

```bash
# From the scripts directory
cd ../scripts/
./deploy-kind.sh

# Or manual Helm installation
helm install ros-ocp ./ros-ocp -n ros-ocp --create-namespace
```

## Chart Structure

```
ros-ocp/
├── Chart.yaml           # Chart metadata
├── values.yaml          # Default configuration values
├── templates/           # Kubernetes resource templates
│   ├── deployments/     # Application deployments
│   ├── statefulsets/    # Stateful services (databases, Kafka)
│   ├── services/        # Service definitions
│   ├── configmaps/      # Configuration management
│   ├── secrets/         # Credential management
│   └── jobs/            # Initialization jobs
└── tests/               # Helm tests
```

## Services Deployed

### Stateful Services
- **PostgreSQL** (3 instances): ROS, Kruize, Sources databases
- **Kafka + Zookeeper**: Message streaming with persistent storage
- **MinIO**: Object storage with persistent volumes

### Application Services
- **Ingress**: File upload API
- **ROS-OCP API**: Main REST API
- **ROS-OCP Processor**: Data processing service
- **ROS-OCP Recommendation Poller**: Kruize integration
- **ROS-OCP Housekeeper**: Maintenance tasks
- **Kruize Autotune**: Optimization recommendation engine
- **Sources API**: Source management
- **Redis**: Caching layer
- **Nginx**: Web server

## Configuration

### Default Values
The chart uses production-ready defaults but can be customized:

```yaml
# Custom values example
global:
  storageClass: "fast-ssd"

resources:
  kruize:
    requests:
      memory: "2Gi"
      cpu: "1000m"
    limits:
      memory: "4Gi"
      cpu: "2000m"
```

### Resource Requirements
Minimum recommended resources:
- **Memory**: 8GB+ (12GB+ recommended)
- **CPU**: 4+ cores
- **Storage**: 10GB+ free disk space

## Access Points

After deployment with NodePort services:

- **Ingress API**: http://localhost:30080
- **ROS-OCP API**: http://localhost:30081
- **Kruize API**: http://localhost:30090
- **MinIO Console**: http://localhost:30099

## Installation Options

The deployment system provides distinct workflows for different use cases:

### 1. CI/CD Validation (KIND-based)
For automated testing and continuous integration using ephemeral KIND clusters. KIND can also be used for local development and testing for vanilla Kubernetes environments, providing a lightweight alternative to full Kubernetes clusters.

```bash
# CI/CD Setup: Create ephemeral KIND cluster
../scripts/deploy-kind.sh

# CI/CD Deploy: Auto-detects Kubernetes platform
../scripts/install-helm-chart.sh

# CI/CD Test: Comprehensive validation
../scripts/test-k8s-dataflow.sh

# CI/CD Cleanup: Destroy entire test environment
../scripts/deploy-kind.sh cleanup --all
```

### 2. OpenShift Production Deployment
For production OpenShift clusters with automatic platform detection. It assumes the OCP cluster is already provisioned with local storage and login has succeeded:

```bash
# OpenShift Deploy: Auto-detects platform and uses OpenShift values
../scripts/install-helm-chart.sh

# OpenShift Test: Platform-specific validation
../scripts/test-ocp-dataflow.sh

# OpenShift Cleanup: Production-safe cleanup options
../scripts/install-helm-chart.sh cleanup --complete
```

### 3. Manual Helm Installation
For advanced use cases or custom configurations:

```bash
# Vanilla Kubernetes with custom values
helm install ros-ocp ./ros-ocp \
  --namespace ros-ocp \
  --create-namespace \
  --values custom-values.yaml

# OpenShift with OpenShift-specific values
helm install ros-ocp ./ros-ocp \
  --namespace ros-ocp \
  --create-namespace \
  -f ../../../openshift-values.yaml
```

### Upgrade
```bash
helm upgrade ros-ocp ./ros-ocp -n ros-ocp
```

## Testing Workflows

### CI/CD Testing (Kubernetes/KIND)
**Purpose**: Automated validation using KIND clusters for continuous integration.

```bash
cd ../scripts/

# Run comprehensive CI/CD validation
./test-k8s-dataflow.sh

# CI/CD-specific operations
./test-k8s-dataflow.sh logs [service-name]
./test-k8s-dataflow.sh cleanup
```

### OpenShift Testing
**Purpose**: Production validation for OpenShift deployments with platform-specific features.

```bash
cd ../scripts/

# Run OpenShift-specific validation
./test-ocp-dataflow.sh

# OpenShift-specific operations
./test-ocp-dataflow.sh logs [service-name]
./test-ocp-dataflow.sh recommendations
./test-ocp-dataflow.sh workloads
```

### Universal Deployment Management
**Purpose**: The `install-helm-chart.sh` script provides consistent deployment across platforms.

**Key Features**:
- **Platform Detection**: Automatically detects Kubernetes vs OpenShift
- **Dynamic Configuration**: Uses appropriate values files based on platform
- **Cluster Detection**: Auto-detects cluster domain and name (OpenShift)
- **Storage Intelligence**: Selects optimal storage classes automatically
- **Conflict Resolution**: Handles Kafka cluster ID conflicts
- **Health Validation**: Comprehensive post-deployment verification

## Troubleshooting

**Check deployment status**:
```bash
helm status ros-ocp -n ros-ocp
kubectl get pods -n ros-ocp
```

**View pod logs**:
```bash
kubectl logs -n ros-ocp -l app.kubernetes.io/name=rosocp-processor
```

**Check persistent volumes**:
```bash
kubectl get pvc -n ros-ocp
```

## Deployment Script Features

### Universal Installation (`install-helm-chart.sh`)
- **Automatic Platform Detection**: Kubernetes vs OpenShift detection
- **Dynamic Values Selection**: Uses values.yaml (K8s) or openshift-values.yaml (OpenShift)
- **Strict Cluster Detection**: Auto-detects cluster name and domain on OpenShift
- **Intelligent Storage**: Auto-selects optimal storage classes per platform
- **Kafka Conflict Resolution**: Prevents cluster ID mismatches automatically
- **Comprehensive Health Checks**: Internal and external connectivity validation
- **Flexible Cleanup**: Standard cleanup (preserves data) or complete cleanup (removes all)

### Testing Scripts
- **`test-k8s-dataflow.sh`**: CI/CD validation for KIND-based Kubernetes clusters
- **`test-ocp-dataflow.sh`**: Production validation for OpenShift with route testing
- **Both support**: Logs viewing, health checks, and cleanup operations

## Chart Features

- **Comprehensive ConfigMaps**: Environment variable management across all services
- **Health Checks**: Proper readiness and liveness probes for all services
- **Persistent Storage**: Configured for all stateful services with platform-specific defaults
- **Resource Management**: Appropriate limits and requests to prevent resource exhaustion
- **Platform Optimization**: OpenShift Routes vs Kubernetes Ingress automatically selected