#!/bin/bash

# KIND Artifacts Cleanup Script
# This script cleans up KIND clusters, containers, and related images
# Can be used standalone or called from other scripts

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-ros-ocp-cluster}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-podman}

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

# Function to detect container runtime
detect_container_runtime() {
    local runtime="${CONTAINER_RUNTIME:-podman}"

    if [ "$runtime" = "auto" ]; then
        if command_exists podman; then
            runtime="podman"
        elif command_exists docker; then
            runtime="docker"
        else
            echo_error "No supported container runtime found. Please install Docker or Podman."
            return 1
        fi
    fi

    if ! command_exists "$runtime"; then
        echo_error "$runtime specified but not found. Please install $runtime."
        return 1
    fi

    export DETECTED_RUNTIME="$runtime"
    echo_info "Using $runtime as container runtime"
    return 0
}

# Function to clean up existing KIND containers and project images
cleanup_kind_artifacts() {
    echo_info "Performing cleanup of KIND artifacts..."

    # Remove KIND cluster if it exists
    if command_exists kind; then
        if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
            echo_info "Removing existing KIND cluster: $KIND_CLUSTER_NAME"
            kind delete cluster --name "$KIND_CLUSTER_NAME" || echo_warning "Failed to delete KIND cluster (may not exist)"
        else
            echo_info "No KIND cluster '$KIND_CLUSTER_NAME' found to remove"
        fi
    else
        echo_warning "kind command not found, skipping cluster cleanup"
    fi

    # Detect container runtime for cleanup
    if ! detect_container_runtime; then
        echo_warning "Could not detect container runtime, skipping container cleanup"
        return 0
    fi

    # Remove KIND containers that might be lingering
    echo_info "Cleaning up lingering KIND containers..."
    if command_exists "$DETECTED_RUNTIME"; then
        # Stop and remove KIND control plane containers
        "$DETECTED_RUNTIME" ps -a --format "{{.Names}}" | grep -E "kind|ros-ocp" | while read -r container; do
            if [ -n "$container" ]; then
                echo_info "Stopping and removing container: $container"
                "$DETECTED_RUNTIME" stop "$container" 2>/dev/null || true
                "$DETECTED_RUNTIME" rm "$container" 2>/dev/null || true
            fi
        done

        # Remove project-related images
        echo_info "Cleaning up project-related images..."
        "$DETECTED_RUNTIME" images --format "{{.Repository}}:{{.Tag}}" | grep -E "ros-ocp-backend|jordigilh" | while read -r image; do
            if [ -n "$image" ]; then
                echo_info "Removing image: $image"
                "$DETECTED_RUNTIME" rmi -f "$image" 2>/dev/null || true
            fi
        done

        # Remove dangling images
        echo_info "Cleaning up dangling images..."
        "$DETECTED_RUNTIME" image prune -f 2>/dev/null || true
    fi

    echo_success "Cleanup completed"
}

# Function to show help
show_help() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Clean up KIND clusters, containers, and related images"
    echo ""
    echo "Options:"
    echo "  --cluster-name NAME    KIND cluster name to clean up (default: ros-ocp-cluster)"
    echo "  --container-runtime     Container runtime to use (default: podman, supports: podman, docker, auto)"
    echo "  --help, -h             Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  KIND_CLUSTER_NAME      Name of KIND cluster (default: ros-ocp-cluster)"
    echo "  CONTAINER_RUNTIME      Container runtime to use (default: podman)"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Clean up default cluster"
    echo "  $0 --cluster-name my-cluster         # Clean up specific cluster"
    echo "  CONTAINER_RUNTIME=docker $0          # Use Docker instead of Podman"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            KIND_CLUSTER_NAME="$2"
            shift 2
            ;;
        --container-runtime)
            CONTAINER_RUNTIME="$2"
            shift 2
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        *)
            echo_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Main execution
echo_info "Starting KIND artifacts cleanup..."
echo_info "Cluster name: $KIND_CLUSTER_NAME"
echo_info "Container runtime: $CONTAINER_RUNTIME"

cleanup_kind_artifacts

echo_success "KIND artifacts cleanup completed successfully!"
