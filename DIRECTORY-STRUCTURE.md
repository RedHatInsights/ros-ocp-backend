# ROS-OCP Backend Directory Structure

This document explains the organized directory structure for better separation of concerns.

## Directory Organization

```
ros-ocp-backend/
├── deployment/          # All deployment-related artifacts
│   ├── docker-compose/  # Docker Compose setup
│   └── kubernetes/      # Kubernetes/Helm deployment
├── docs/                # Documentation
└── scripts/             # Original location (preserved for backward compatibility)
    ├── .env             # Environment variables (original)
    ├── docker-compose.yml          # Original Docker Compose (preserved)
    ├── cdappconfig.json # Original Kruize config (preserved)
    ├── get_kruize_image_tag.py     # Utility script (preserved)
    ├── ros_ocp_backend.postman_collection.json  # API collection (preserved)
    └── samples/         # Original sample data (preserved)
```

## Directory Purposes

### 📦 `deployment/`
All deployment-related artifacts organized by deployment method:

- **`docker-compose/`** - Complete Docker Compose setup for local development
  - `docker-compose.yml` - Base service definitions
  - `docker-compose.override.yml` - Local development overrides
  - `test-ros-ocp-dataflow.sh` - End-to-end Docker Compose testing

- **`kubernetes/`** - Kubernetes deployment using Helm
  - `helm/ros-ocp/` - Helm chart (renamed from ros-ocp-helm)
  - `scripts/deploy-kind.sh` - KIND cluster setup script
  - `scripts/install-helm-chart.sh` - Helm chart deployment script (works with any cluster)
  - `scripts/test-k8s-dataflow.sh` - End-to-end Kubernetes testing
  - `docs/KUBERNETES-QUICKSTART.md` - Complete Kubernetes guide

### 📚 `docs/`
Centralized documentation:
- `README.md` - Original scripts documentation (moved)
- `ROS-OCP-DATAFLOW.md` - Data flow documentation

## Quick Start Paths

### Kubernetes Deployment
```bash
# Two-step deployment to KIND cluster
cd deployment/kubernetes/scripts/
./deploy-kind.sh           # Setup KIND cluster
./install-helm-chart.sh    # Deploy ROS-OCP
./test-k8s-dataflow.sh     # Test deployment
```

### Docker Compose Deployment
```bash
# Start services
cd deployment/docker-compose/
podman-compose up -d

# Test the deployment
./test-ros-ocp-dataflow.sh
```

## Benefits of This Structure

1. **Clear Separation of Concerns** - Each directory has a single, well-defined purpose
2. **Better Discoverability** - Users can quickly find what they need based on their task
3. **Scalability** - Easy to add new deployment methods or testing approaches
4. **Professional Organization** - Follows standard project structure patterns
5. **Maintainability** - Changes to one area don't affect others

## Migration Notes

- **New organized structure** created in separate directories for better organization
- **Original scripts/ directory preserved** with all original files from commit d34b187d91a59e6b42d7abcd6bdf5747a7684a07
- **Backward compatibility maintained** - all original functionality remains accessible
- **Duplication by design** - allows easier merging with upstream changes
- Path references in `deploy-kind.sh` updated to reference `../helm/ros-ocp`
- Future development should use the new organized structure, not the original scripts/ directory

## Access Points by Deployment Method

| Service | Kubernetes | Docker Compose | Description |
|---------|------------|----------------|-------------|
| **Ingress API** | http://localhost:30080 | http://localhost:3000 | File upload endpoint |
| **ROS-OCP API** | http://localhost:30081 | http://localhost:8001 | Main REST API |
| **Kruize API** | http://localhost:30090 | http://localhost:8080 | Optimization engine |
| **MinIO Console** | http://localhost:30099 | http://localhost:9990 | Storage admin UI |

## Support

For deployment-specific help:
- **Kubernetes**: See `deployment/kubernetes/docs/KUBERNETES-QUICKSTART.md`
- **Docker Compose**: See `deployment/docker-compose/README.md`
- **Testing**: See individual README files in `testing/` subdirectories