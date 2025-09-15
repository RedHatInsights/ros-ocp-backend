#!/bin/bash

# ROS-OCP KIND Cluster Setup Script
# This script sets up a KIND cluster for ROS-OCP deployment
# For Helm chart deployment, use ./install-helm-chart.sh
# Container Runtime: Docker (default)
#
# MEMORY REQUIREMENTS:
# - Docker Desktop: Minimum 6GB memory allocation required
# - KIND node: Fixed 6GB memory limit for deterministic deployment
# - Allocatable: ~5.2GB after system reservations
# - Full deployment: ~4.5GB for all ROS-OCP services

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-ros-ocp-cluster}

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
    local runtime="$CONTAINER_RUNTIME"

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

# Function to check prerequisites
check_prerequisites() {
    echo_info "Checking prerequisites..."

    local missing_tools=()

    if ! command_exists kind; then
        missing_tools+=("kind")
    fi

    if ! command_exists kubectl; then
        missing_tools+=("kubectl")
    fi

    if ! command_exists helm; then
        missing_tools+=("helm")
    fi

    if ! detect_container_runtime; then
        return 1
    fi

    if [ ${#missing_tools[@]} -gt 0 ]; then
        echo_error "Missing required tools: ${missing_tools[*]}"
        echo_info "Please install the missing tools:"

        for tool in "${missing_tools[@]}"; do
            case $tool in
                "kind")
                    echo_info "  Install KIND: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install kind"
                    fi
                    ;;
                "kubectl")
                    echo_info "  Install kubectl: https://kubernetes.io/docs/tasks/tools/"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install kubectl"
                    fi
                    ;;
                "helm")
                    echo_info "  Install Helm: https://helm.sh/docs/intro/install/"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install helm"
                    fi
                    ;;
                "docker")
                    echo_info "  Install Docker: https://docs.docker.com/get-docker/"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install --cask docker"
                    fi
                    ;;
            esac
        done

        return 1
    fi

    # Check container runtime is running
    if [ "$DETECTED_RUNTIME" = "docker" ]; then
        if ! docker info >/dev/null 2>&1; then
            echo_error "Docker is not running. Please start Docker and try again."
            return 1
        fi
    elif [ "$DETECTED_RUNTIME" = "podman" ]; then
        if ! podman info >/dev/null 2>&1; then
            echo_error "Podman is not accessible. Please ensure Podman is running."
            return 1
        fi
        export KIND_EXPERIMENTAL_PROVIDER=podman

        # Warn about PID limits
        if ! grep -q "pids_limit.*=.*0" /etc/containers/containers.conf 2>/dev/null; then
            echo_warning "Podman may encounter PID limit issues with nginx-ingress controller"
            echo_info "To fix this, create or edit /etc/containers/containers.conf and add:"
            echo_info "  [containers]"
            echo_info "  pids_limit = 0"
            echo_info ""
            echo_info "This removes PID limits for containers, allowing nginx to start properly."
            echo_info "After editing, restart your session or run: systemctl --user restart podman.socket"
        fi
    fi

    echo_success "All prerequisites are installed"
    return 0
}

# Function to create KIND cluster with storage
create_kind_cluster() {
    echo_info "Creating KIND cluster: $KIND_CLUSTER_NAME"

    # Check if cluster already exists
    if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        echo_error "KIND cluster '$KIND_CLUSTER_NAME' already exists"
        echo_info "Please delete the existing cluster first with:"
        echo_info "  kind delete cluster --name $KIND_CLUSTER_NAME"
        echo_info "Or use a different cluster name by setting KIND_CLUSTER_NAME environment variable"
        exit 1
    fi

    # Create KIND cluster configuration - using the most common approach
    local kind_config=$(cat <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${KIND_CLUSTER_NAME}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 30080
    protocol: TCP
EOF
)

    # Create cluster with standard configuration
    echo "$kind_config" | KIND_EXPERIMENTAL_DOCKER_NETWORK="" kind create cluster --config=-

    if [ $? -ne 0 ]; then
        echo_error "Failed to create KIND cluster"
        return 1
    fi
    echo_success "KIND cluster '$KIND_CLUSTER_NAME' created successfully"

    # Wait a moment for the container to fully initialize before applying memory constraints
    echo_info "Waiting for KIND container to initialize..."
    sleep 10

    # Set memory limit on the KIND node container to 6GB for deterministic deployment
    echo_info "Configuring KIND node with 6GB memory limit..."
    if docker update --memory=6g "${KIND_CLUSTER_NAME}-control-plane" >/dev/null 2>&1; then
        echo_success "Memory limit set to 6GB"
        # Give the container a moment to adjust to the new memory limit
        sleep 5
    else
        echo_warning "Could not set 6GB memory limit on KIND container."
        echo_warning "This may cause deployment issues if Docker has insufficient memory."
        echo_info "Continuing with default Docker memory allocation..."

        # Check actual Docker memory available
        local actual_memory=$(docker system info --format '{{.MemTotal}}' 2>/dev/null || echo "0")
        if [ "$actual_memory" -gt 0 ]; then
            local actual_gb=$((actual_memory / 1024 / 1024 / 1024))
            echo_info "Docker has ${actual_gb}GB memory available"
            if [ "$actual_gb" -lt 5 ]; then
                echo_error "Docker has less than 5GB memory. Deployment may fail due to resource constraints."
                echo_error "Please increase Docker memory allocation and try again."
                return 1
            fi
        fi
    fi

    # Set kubectl context
    kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}"
    echo_success "kubectl context set to kind-${KIND_CLUSTER_NAME}"

    # Wait for API server to be fully ready with extended timeout for 6GB constrained environment
    echo_info "Waiting for API server to be fully ready..."

    # First check if the KIND container is running
    if ! docker ps --filter "name=${KIND_CLUSTER_NAME}-control-plane" --filter "status=running" | grep -q "${KIND_CLUSTER_NAME}-control-plane"; then
        echo_error "KIND container ${KIND_CLUSTER_NAME}-control-plane is not running"
        docker ps --filter "name=${KIND_CLUSTER_NAME}-control-plane"
        return 1
    fi

    local retries=60  # Increased to 5 minutes for memory-constrained environment
    local count=0
    while [ $count -lt $retries ]; do
        if kubectl get --raw /healthz >/dev/null 2>&1; then
            echo_success "API server is ready"
            break
        fi

        # Show progress every 10 attempts (50 seconds)
        if [ $((count % 10)) -eq 0 ] && [ $count -gt 0 ]; then
            echo_info "Still waiting for API server... (${count}/${retries} - $((count * 5 / 60))m ${count * 5 % 60}s elapsed)"
            # Show container status for debugging
            echo_info "KIND container status: $(docker inspect --format='{{.State.Status}}' ${KIND_CLUSTER_NAME}-control-plane 2>/dev/null || echo 'unknown')"
        else
            echo_info "Waiting for API server... ($((count + 1))/$retries)"
        fi

        sleep 5
        count=$((count + 1))
    done

    if [ $count -eq $retries ]; then
        echo_error "API server not ready after $retries attempts (5 minutes)"
        echo_error "Debugging information:"
        echo_info "KIND container status:"
        docker ps --filter "name=${KIND_CLUSTER_NAME}-control-plane"
        echo_info "KIND container logs (last 20 lines):"
        docker logs --tail 20 "${KIND_CLUSTER_NAME}-control-plane" 2>/dev/null || echo "Could not retrieve container logs"
        return 1
    fi
}

# Function to install storage provisioner
install_storage_provisioner() {
    echo_info "Installing storage provisioner..."

    # KIND comes with Rancher Local Path Provisioner by default
    # We just need to make it the default storage class
    kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

    echo_success "Storage provisioner configured"
}

# Function to install NGINX Ingress Controller
install_ingress_controller() {
    echo_info "Installing NGINX Ingress Controller..."

    # Install NGINX Ingress Controller
        # Use cloud deployment + NodePort patch for Podman compatibility
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
    sleep 5
    kubectl patch service ingress-nginx-controller -n ingress-nginx --type='json' -p='[
        {"op": "replace", "path": "/spec/type", "value": "NodePort"},
        {"op": "add", "path": "/spec/ports/1/nodePort", "value": 30443}
    ]'

    # Wait for the deployment to be created
    echo_info "Waiting for ingress controller deployment to be created..."
    local retries=30
    local count=0
    while [ $count -lt $retries ]; do
        if kubectl get deployment ingress-nginx-controller -n ingress-nginx >/dev/null 2>&1; then
            echo_success "Ingress controller deployment found"
            break
        fi
        echo_info "Waiting for deployment... ($((count + 1))/$retries)"
        sleep 2
        count=$((count + 1))
    done

    # Wait for admission webhook job to complete
    echo_info "Waiting for admission webhook setup to complete..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=complete job/ingress-nginx-admission-create \
        --timeout=120s || true

    # Wait for admission webhook patch job if it exists
    kubectl wait --namespace ingress-nginx \
        --for=condition=complete job/ingress-nginx-admission-patch \
        --timeout=60s || true


    # Enable debug logging in NGINX ingress controller
    echo_info "Enabling debug logs in NGINX ingress controller..."
    kubectl patch deployment ingress-nginx-controller -n ingress-nginx --type='json' -p='[
        {
            "op": "add",
            "path": "/spec/template/spec/containers/0/args/-",
            "value": "--v=2"
        },
        {
            "op": "add",
            "path": "/spec/template/spec/containers/0/args/-",
            "value": "--logtostderr=true"
        }
    ]' || echo_warning "Debug logging patch failed, continuing..."

    # Give time for the patch to take effect
    sleep 5

    # Wait for ingress controller to be ready
    echo_info "Waiting for NGINX Ingress Controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=300s

    echo_success "NGINX Ingress Controller is ready"
}




# Note: Using localhost directly - no hostname setup required

    # Check if Helm chart directory exists
    if [ ! -d "../helm/ros-ocp" ]; then
        echo_error "Helm chart directory not found: ../helm/ros-ocp"
        return 1
    fi

    # Install or upgrade the Helm release
    helm upgrade --install "$HELM_RELEASE_NAME" ../helm/ros-ocp \
        --namespace "$NAMESPACE" \
        --create-namespace \
        --set global.storageClass="$STORAGE_CLASS" \
        --timeout=600s \
        --wait

    if [ $? -eq 0 ]; then
        echo_success "Helm chart deployed successfully"
    else
        echo_error "Failed to deploy Helm chart"
        return 1
    fi
}

# Function to wait for pods to be ready
wait_for_pods() {
    echo_info "Waiting for pods to be ready..."

    # Wait for all pods to be ready (excluding jobs)
    kubectl wait --for=condition=ready pod -l "app.kubernetes.io/instance=$HELM_RELEASE_NAME" \
        --namespace "$NAMESPACE" \
        --timeout=600s \
        --field-selector=status.phase!=Succeeded

    echo_success "All pods are ready"
}

# Function to create NodePort services for external access
create_nodeport_services() {
    echo_info "Creating NodePort services for external access..."

    # Ingress service
    kubectl patch service "${HELM_RELEASE_NAME}-ingress" -n "$NAMESPACE" \
        -p '{"spec":{"type":"NodePort","ports":[{"port":3000,"nodePort":30083,"targetPort":"http","protocol":"TCP","name":"http"}]}}'

    # ROS-OCP API service
    kubectl patch service "${HELM_RELEASE_NAME}-rosocp-api" -n "$NAMESPACE" \
        -p '{"spec":{"type":"NodePort","ports":[{"port":8000,"nodePort":30081,"targetPort":"http","protocol":"TCP","name":"http"},{"port":9000,"nodePort":30082,"targetPort":"metrics","protocol":"TCP","name":"metrics"}]}}'

    # Kruize service
    kubectl patch service "${HELM_RELEASE_NAME}-kruize" -n "$NAMESPACE" \
        -p '{"spec":{"type":"NodePort","ports":[{"port":8080,"nodePort":30090,"targetPort":"http","protocol":"TCP","name":"http"}]}}'

    # MinIO service (API and Console)
    kubectl patch service "${HELM_RELEASE_NAME}-minio" -n "$NAMESPACE" \
        --type='json' \
        -p='[
          {"op": "replace", "path": "/spec/type", "value": "NodePort"},
          {"op": "add", "path": "/spec/ports/0/nodePort", "value": 30091},
          {"op": "add", "path": "/spec/ports/1/nodePort", "value": 30099}
        ]'

    echo_success "NodePort services created"
}

# Function to show deployment status
show_status() {
    echo_info "KIND Cluster Status"
    echo_info "==================="

    echo_info "Cluster: kind-${KIND_CLUSTER_NAME}"
    echo_info "Context: $(kubectl config current-context)"
    echo ""

    echo_info "Cluster Info:"
    kubectl cluster-info
    echo ""

    echo_info "Nodes:"
    kubectl get nodes -o wide
    echo ""

    echo_info "Storage Classes:"
    kubectl get storageclass
    echo ""

    echo_info "Access Points:"
    echo_info "  - Nginx Ingress: http://localhost:30080 (404 response is normal - no ingress rules configured)"

    # Get the actual port used for ros-ocp-ingress
    local ros_ingress_port
    ros_ingress_port=$(kubectl get service "${HELM_RELEASE_NAME}-ingress" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "30083")
    echo_info "  - ROS-OCP Ingress: http://localhost:${ros_ingress_port}"
    echo_info "  - Ingress API: http://localhost:${ros_ingress_port}/api/ingress/v1/version"
    echo_info "  - ROS-OCP API: http://localhost:30081/status"
    echo_info "  - Kruize API: http://localhost:30090/listPerformanceProfiles"
    echo_info "  - MinIO API: http://localhost:30091 (S3 API)"
    echo_info "  - MinIO Console: http://localhost:30099 (Web UI - minioaccesskey/miniosecretkey)"
    echo_info "Ingress Controller:"
    kubectl get pods -n ingress-nginx
    echo ""

    echo_info "Useful Commands:"
    echo_info "  - Deploy Helm chart: ./install-helm-chart.sh"
    echo_info "  - Test deployment: ./test-k8s-dataflow.sh"
    echo_info "  - Delete cluster: kind delete cluster --name $KIND_CLUSTER_NAME"
}

# Function to run health checks
run_health_checks() {
    echo_info "Running health checks..."

    local failed_checks=0

    # Check if nginx ingress controller is accessible
    local nginx_response
    nginx_response=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:30080/" 2>/dev/null || echo "000")

    if [ "$nginx_response" = "000" ] || [ -z "$nginx_response" ]; then
        echo_error "Nginx Ingress is not accessible on port 30080"
        failed_checks=$((failed_checks + 1))
    else
        echo_success "Ingress API is accessible on port 30080 (HTTP ${nginx_response})"
    fi

    # Check if ROS-OCP ingress API is accessible
    if curl -f -s "http://localhost:30083/api/ingress/v1/version" >/dev/null 2>&1; then
        echo_success "ROS-OCP Ingress API is accessible on port 30083"
    else
        echo_error "ROS-OCP Ingress API is not accessible on port 30083"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if ROS-OCP API is accessible
    if curl -f -s http://localhost:30081/status >/dev/null; then
        echo_success "ROS-OCP API is accessible"
    else
        echo_error "ROS-OCP API is not accessible"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if Kruize is accessible
    if curl -f -s http://localhost:30090/listPerformanceProfiles >/dev/null; then
        echo_success "Kruize API is accessible"
    else
        echo_error "Kruize API is not accessible"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if MinIO console is accessible
    if curl -f -s http://localhost:30099/ >/dev/null; then
        echo_success "MinIO console is accessible"
    else
        echo_error "MinIO console is not accessible"
        failed_checks=$((failed_checks + 1))
    fi

    if [ $failed_checks -eq 0 ]; then
        echo_success "All health checks passed!"
    else
        echo_warning "$failed_checks health check(s) failed"
    fi

    return $failed_checks
}

# Function to cleanup
cleanup() {
    if [ "${1:-}" = "--all" ]; then
        echo_info "Deleting KIND cluster..."
        kind delete cluster --name "$KIND_CLUSTER_NAME"
        echo_success "KIND cluster deleted"
    else
        echo_warning "This script only manages the KIND cluster."
        echo_info "For Helm deployment cleanup, use: ./install-helm-chart.sh cleanup"
        echo_info "To delete the entire cluster, run: $0 cleanup --all"
    fi
}

# Main execution
main() {
    echo_info "ROS-OCP KIND Cluster Setup"
    echo_info "=========================="
    echo_info "This script sets up a KIND cluster for ROS-OCP deployment."
    echo_info "For Helm chart deployment, use: ./install-helm-chart.sh"
    echo ""

    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi

    echo_info "Configuration:"
    echo_info "  KIND Cluster: $KIND_CLUSTER_NAME"
    echo_info "  Helm Release: $HELM_RELEASE_NAME"
    echo_info "  Namespace: $NAMESPACE"
    echo_info "  Storage Class: $STORAGE_CLASS"
    echo_info "  Container Runtime: $DETECTED_RUNTIME"
    echo ""

    # Create KIND cluster
    if ! create_kind_cluster; then
        exit 1
    fi

    # Install storage provisioner
    if ! install_storage_provisioner; then
        exit 1
    fi

    # Install ingress controller
    if ! install_ingress_controller; then
        exit 1
    fi

    # Show cluster status
    show_status

    echo ""
    echo_success "KIND cluster setup completed!"
    echo_info "The cluster '$KIND_CLUSTER_NAME' is now ready for Helm chart deployment"
    echo_info ""
    echo_info "Next Steps:"
    echo_info "  1. Deploy ROS-OCP Helm chart: ./install-helm-chart.sh"
    echo_info "  2. Test the deployment: ./test-k8s-dataflow.sh"
}

# Handle script arguments
case "${1:-}" in
    "cleanup")
        cleanup "${2:-}"
        exit 0
        ;;
    "status")
        show_status
        exit 0
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  (none)          - Setup KIND cluster for ROS-OCP"
        echo "  cleanup --all   - Delete entire KIND cluster"
        echo "  status          - Show cluster status"
        echo "  help            - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  KIND_CLUSTER_NAME - Name of KIND cluster (default: ros-ocp-cluster)"
        echo ""
        echo "Requirements:"
        echo "  - Docker must be running (default container runtime)"
        echo "  - kubectl and kind must be installed"
        echo "  - Docker Desktop: Minimum 6GB memory allocation"
        echo ""
        echo "Two-Step Deployment:"
        echo "  1. ./deploy-kind.sh       - Setup KIND cluster"
        echo "  2. ./install-helm-chart.sh - Deploy ROS-OCP Helm chart"
        echo ""
        echo "Next Steps:"
        echo "  After successful setup, run ./install-helm-chart.sh to deploy ROS-OCP"
        exit 0
        ;;
esac

# Run main function
main "$@"