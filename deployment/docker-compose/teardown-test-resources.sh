#!/bin/bash

# ROS-OCP Backend Test Resources Teardown Script
# This script tears down all resources created by the test setup:
# - Docker-compose services
# - KIND cluster
# - Authentication files

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CLUSTER_NAME="ros-ingress-dev"
KUBECONFIG_FILE="/tmp/ros-ingress-kubeconfig"
AUTH_ENV_FILE="/Users/masayag/dev/insights-onprem/ros-ocp-backend/scripts/.ingress-auth.env"

echo_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

echo_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

echo_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

echo_info "Starting ROS-OCP Backend Test Resources Teardown"
echo_info "=============================================="

cd "$SCRIPT_DIR"

# 1. Stop and remove docker-compose services
echo_info "=== STEP 1: Stopping Docker-Compose Services ==="
if [ -f "docker-compose.yml" ]; then
    echo_info "Stopping all docker-compose services..."
    if command_exists podman-compose; then
        podman-compose down --volumes --remove-orphans
        echo_success "Docker-compose services stopped and removed"
    elif command_exists docker-compose; then
        echo_warning "Using docker-compose as fallback (podman-compose not found)"
        docker-compose down --volumes --remove-orphans
        echo_success "Docker-compose services stopped and removed"
    else
        echo_error "Neither podman-compose nor docker-compose found"
        exit 1
    fi
else
    echo_warning "docker-compose.yml not found in current directory"
fi

# 2. Remove any leftover containers
echo_info "=== STEP 2: Cleaning Up Leftover Containers ==="
if command_exists podman; then
    echo_info "Removing any leftover ROS-OCP containers..."
    
    # Stop and remove containers that might be running
    for container in rosocp-api_1 rosocp-processor_1 rosocp-recommendation-poller_1 rosocp-housekeeper_1 ingress_1 sources-api-go_1 kruize-autotune_1 minio_1 kafka_1 zookeeper_1 redis_1 nginx_1; do
        if podman ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
            echo_info "Removing container: $container"
            podman rm -f "$container" 2>/dev/null || true
        fi
    done
    
    # Remove containers by name pattern
    echo_info "Removing containers with 'ros' or 'insight' in the name..."
    podman ps -a --format "{{.Names}}" | grep -E "(ros|insight|kruize|minio)" | xargs -r podman rm -f 2>/dev/null || true
    
    echo_success "Container cleanup completed"
elif command_exists docker; then
    echo_warning "Using docker as fallback (podman not found)"
    echo_info "Removing any leftover ROS-OCP containers..."
    
    # Stop and remove containers that might be running
    for container in rosocp-api_1 rosocp-processor_1 rosocp-recommendation-poller_1 rosocp-housekeeper_1 ingress_1 sources-api-go_1 kruize-autotune_1 minio_1 kafka_1 zookeeper_1 redis_1 nginx_1; do
        if docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
            echo_info "Removing container: $container"
            docker rm -f "$container" 2>/dev/null || true
        fi
    done
    
    echo_success "Container cleanup completed"
else
    echo_warning "Neither podman nor docker found, skipping container cleanup"
fi

# 3. Remove KIND cluster
echo_info "=== STEP 3: Removing KIND Cluster ==="
if command_exists kind; then
    echo_info "Checking for KIND cluster: $KIND_CLUSTER_NAME"
    if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
        echo_info "Deleting KIND cluster: $KIND_CLUSTER_NAME"
        kind delete cluster --name "$KIND_CLUSTER_NAME"
        echo_success "KIND cluster deleted: $KIND_CLUSTER_NAME"
    else
        echo_info "KIND cluster not found: $KIND_CLUSTER_NAME"
    fi
else
    echo_warning "KIND not found, skipping cluster cleanup"
fi

# 4. Clean up authentication files
echo_info "=== STEP 4: Cleaning Up Authentication Files ==="

# Remove kubeconfig file
if [ -f "$KUBECONFIG_FILE" ]; then
    echo_info "Removing kubeconfig file: $KUBECONFIG_FILE"
    rm -f "$KUBECONFIG_FILE"
    echo_success "Kubeconfig file removed"
else
    echo_info "Kubeconfig file not found: $KUBECONFIG_FILE"
fi

# Remove authentication environment file
if [ -f "$AUTH_ENV_FILE" ]; then
    echo_info "Removing authentication environment file: $AUTH_ENV_FILE"
    rm -f "$AUTH_ENV_FILE"
    echo_success "Authentication environment file removed"
else
    echo_info "Authentication environment file not found: $AUTH_ENV_FILE"
fi

# Remove README file created by setup
README_FILE="/Users/masayag/dev/insights-onprem/ros-ocp-backend/scripts/README-AUTH-SETUP.md"
if [ -f "$README_FILE" ]; then
    echo_info "Removing authentication setup README: $README_FILE"
    rm -f "$README_FILE"
    echo_success "Authentication setup README removed"
fi

# 5. Clean up local data volumes
echo_info "=== STEP 5: Cleaning Up Local Data ==="

# Remove MinIO data directory if it exists
MINIO_DATA_DIR="$SCRIPT_DIR/minio-data"
if [ -d "$MINIO_DATA_DIR" ]; then
    echo_info "Removing MinIO data directory: $MINIO_DATA_DIR"
    rm -rf "$MINIO_DATA_DIR"
    echo_success "MinIO data directory removed"
else
    echo_info "MinIO data directory not found: $MINIO_DATA_DIR"
fi

# 6. Clean up networks (if using podman)
echo_info "=== STEP 6: Cleaning Up Networks ==="
if command_exists podman; then
    echo_info "Removing docker-compose networks..."
    podman network ls --format "{{.Name}}" | grep -E "(docker-compose|ros)" | xargs -r podman network rm 2>/dev/null || true
    echo_success "Network cleanup completed"
elif command_exists docker; then
    echo_info "Removing docker-compose networks..."
    docker network ls --format "{{.Name}}" | grep -E "(docker-compose|ros)" | xargs -r docker network rm 2>/dev/null || true
    echo_success "Network cleanup completed"
fi

# 7. Summary
echo_info "=== TEARDOWN SUMMARY ==="
echo_success "✓ Docker-compose services stopped and removed"
echo_success "✓ Leftover containers cleaned up"
echo_success "✓ KIND cluster removed"
echo_success "✓ Authentication files cleaned up"
echo_success "✓ Local data directories removed"
echo_success "✓ Networks cleaned up"

echo ""
echo_success "🎉 All test resources have been successfully torn down!"
echo_info "Your system is now clean and ready for a fresh test setup."

# Optional: Show what's still running
echo ""
echo_info "=== REMAINING CONTAINERS ==="
if command_exists podman; then
    echo_info "Currently running containers:"
    podman ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" || echo_info "No containers running"
elif command_exists docker; then
    echo_info "Currently running containers:"
    docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" || echo_info "No containers running"
fi