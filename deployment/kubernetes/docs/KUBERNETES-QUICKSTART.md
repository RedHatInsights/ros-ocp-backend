# ROS-OCP Quick Start Guide

This guide walks you through deploying and testing the ROS-OCP backend services on both Kubernetes and OpenShift clusters using the Helm chart from the [ros-helm-chart repository](https://github.com/insights-onprem/ros-helm-chart).

## Helm Chart Location

The ROS-OCP Helm chart is maintained in a separate repository: **[insights-onprem/ros-helm-chart](https://github.com/insights-onprem/ros-helm-chart)**

### Deployment Methods
For the most up-to-date deployment guide, please refer to the **[ros-helm-chart quickstart documentation](https://github.com/insights-onprem/ros-helm-chart/blob/main/docs/quickstart.md)**.

## Quick Start

### Prerequisites
- kubectl configured with target cluster
- helm installed  
- For KIND: Docker or Podman installed
- For OpenShift: oc CLI configured

### Deployment Options

#### Option 1: Kubernetes/KIND Development
```bash
# For KIND development cluster
git clone https://github.com/insights-onprem/ros-helm-chart.git
cd ros-helm-chart/scripts/

# Step 1: Setup KIND cluster
./deploy-kind.sh

# Step 2: Deploy ROS-OCP services
./install-helm-chart.sh
```

#### Option 2: OpenShift Production
```bash
# For OpenShift clusters
git clone https://github.com/insights-onprem/ros-helm-chart.git
cd ros-helm-chart/scripts/

# Deploy directly to OpenShift (script auto-detects platform)
./install-helm-chart.sh

# Or use local chart for development
USE_LOCAL_CHART=true LOCAL_CHART_PATH=../ros-ocp ./install-helm-chart.sh
```

## End-to-End Testing

After deployment, test the complete data pipeline:

```bash
# Clone ros-ocp-backend for testing scripts
git clone https://github.com/gciavarrini/ros-ocp-backend.git
cd ros-ocp-backend/deployment/kubernetes/scripts/

# For Kubernetes/KIND deployments
./test-k8s-dataflow.sh

# For OpenShift deployments  
./test-ocp-dataflow.sh

# Run health checks only
./test-k8s-dataflow.sh health    # Kubernetes
./test-ocp-dataflow.sh health    # OpenShift
```

## Documentation

For comprehensive documentation, see:
- **[ROS Helm Chart Quickstart](https://github.com/insights-onprem/ros-helm-chart/blob/main/docs/quickstart.md)** - Complete deployment guide
- **[ROS Helm Chart Troubleshooting](https://github.com/insights-onprem/ros-helm-chart/blob/main/docs/troubleshooting.md)** - Common issues and solutions

## Support

For deployment issues:
- Check the [ros-helm-chart repository](https://github.com/insights-onprem/ros-helm-chart) for the latest documentation
- Review test output from `./test-k8s-dataflow.sh`
- Check pod logs: `kubectl logs -n ros-ocp <pod-name>`
