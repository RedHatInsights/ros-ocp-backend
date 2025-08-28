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

### Development (KIND)
```bash
# Use the deployment script
../scripts/deploy-kind.sh
```

### Production Cluster
```bash
# Install with custom values
helm install ros-ocp ./ros-ocp \
  --namespace ros-ocp \
  --create-namespace \
  --values production-values.yaml
```

### Upgrade
```bash
helm upgrade ros-ocp ./ros-ocp -n ros-ocp
```

## Testing

After deployment, run end-to-end testing:

```bash
cd ../scripts/
./test-k8s-dataflow.sh
```

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

## Notes

- The chart includes comprehensive ConfigMaps for environment variable management
- All services include proper health checks and readiness probes
- Persistent storage is configured for stateful services
- Resource limits are set to prevent resource exhaustion