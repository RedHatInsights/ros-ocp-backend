#!/bin/bash

# ROS-OCP Kubernetes Deployment Script for KIND
# This script deploys the ROS-OCP Helm chart on a KIND cluster with proper storage and dependencies
# Container Runtime: Auto-detects Docker or Podman (configurable via CONTAINER_RUNTIME)

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
HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-ros-ocp}
NAMESPACE=${NAMESPACE:-ros-ocp}
STORAGE_CLASS=${STORAGE_CLASS:-standard}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-auto}

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

    # Create KIND cluster configuration with increased file descriptor limits
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
  - |
    kind: KubeletConfiguration
    maxPods: 250
    systemReserved:
      cpu: "0.5"
      memory: "1Gi"
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
  - containerPort: 30081
    hostPort: 30081
    protocol: TCP
  - containerPort: 30082
    hostPort: 30082
    protocol: TCP
  - containerPort: 30090
    hostPort: 30090
    protocol: TCP
  - containerPort: 30091
    hostPort: 30091
    protocol: TCP
  - containerPort: 30099
    hostPort: 30099
    protocol: TCP
  - containerPort: 30443
    hostPort: 30443
    protocol: TCP
  - containerPort: 30083
    hostPort: 30083
    protocol: TCP
EOF
)

    echo "$kind_config" | kind create cluster --config=-

    if [ $? -eq 0 ]; then
        echo_success "KIND cluster '$KIND_CLUSTER_NAME' created successfully"
    else
        echo_error "Failed to create KIND cluster"
        return 1
    fi

    # Set kubectl context
    kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}"
    echo_success "kubectl context set to kind-${KIND_CLUSTER_NAME}"

    # Wait for API server to be fully ready
    echo_info "Waiting for API server to be fully ready..."
    local retries=30
    local count=0
    while [ $count -lt $retries ]; do
        if kubectl get --raw /healthz >/dev/null 2>&1; then
            echo_success "API server is ready"
            break
        fi
        echo_info "Waiting for API server... ($((count + 1))/$retries)"
        sleep 5
        count=$((count + 1))
    done

    if [ $count -eq $retries ]; then
        echo_error "API server not ready after $retries attempts"
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
    if [ "$DETECTED_RUNTIME" = "podman" ]; then
        # Use cloud deployment + NodePort patch for Podman compatibility
        kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
        sleep 5
        kubectl patch service ingress-nginx-controller -n ingress-nginx --type='json' -p='[
            {"op": "replace", "path": "/spec/type", "value": "NodePort"},
            {"op": "add", "path": "/spec/ports/0/nodePort", "value": 30080},
            {"op": "add", "path": "/spec/ports/1/nodePort", "value": 30443}
        ]'
    else
        # Use KIND-specific deployment for Docker
        kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/kind/deploy.yaml
    fi

    # Wait additional time for API server to be fully stable
    echo_info "Waiting for API server to be fully stable before ingress controller starts..."
    sleep 30

    # Wait for the deployment to be created before patching
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

    # Wait for admission webhook job to complete (creates the certificate secret)
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
}

# Function to create namespace
create_namespace() {
    echo_info "Creating namespace: $NAMESPACE"

    if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
        echo_warning "Namespace '$NAMESPACE' already exists"
    else
        kubectl create namespace "$NAMESPACE"
        echo_success "Namespace '$NAMESPACE' created"
    fi
}

# Function to deploy Helm chart
deploy_helm_chart() {
    echo_info "Deploying ROS-OCP Helm chart..."

    cd "$SCRIPT_DIR"

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
    echo_info "Deployment Status"
    echo_info "=================="

    echo_info "Cluster: kind-${KIND_CLUSTER_NAME}"
    echo_info "Namespace: $NAMESPACE"
    echo_info "Helm Release: $HELM_RELEASE_NAME"
    echo ""

    echo_info "Pods:"
    kubectl get pods -n "$NAMESPACE" -o wide
    echo ""

    echo_info "Services:"
    kubectl get services -n "$NAMESPACE"
    echo ""

    echo_info "Storage:"
    kubectl get pvc -n "$NAMESPACE"
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
    echo ""

    echo_info "Useful Commands:"
    echo_info "  - View logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/instance=$HELM_RELEASE_NAME"
    echo_info "  - Port forward ingress: kubectl port-forward -n $NAMESPACE svc/${HELM_RELEASE_NAME}-ingress 3000:3000"
    echo_info "  - Port forward API: kubectl port-forward -n $NAMESPACE svc/${HELM_RELEASE_NAME}-rosocp-api 8001:8000"
    echo_info "  - Delete deployment: helm uninstall $HELM_RELEASE_NAME -n $NAMESPACE"
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
    echo_info "Cleaning up..."

    if [ "${1:-}" = "--all" ]; then
        echo_info "Deleting KIND cluster..."
        kind delete cluster --name "$KIND_CLUSTER_NAME"
        echo_success "KIND cluster deleted"
    else
        echo_info "Deleting Helm release..."
        helm uninstall "$HELM_RELEASE_NAME" -n "$NAMESPACE" || true
        echo_info "Deleting namespace..."
        kubectl delete namespace "$NAMESPACE" || true
        echo_success "Helm release and namespace deleted"
        echo_info "To delete the entire cluster, run: $0 cleanup --all"
    fi
}

# Main execution
main() {
    echo_info "ROS-OCP Kubernetes Deployment for KIND"
    echo_info "======================================="

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

    # Create namespace
    if ! create_namespace; then
        exit 1
    fi

    # Deploy Helm chart
    if ! deploy_helm_chart; then
        exit 1
    fi

    # Wait for pods to be ready
    if ! wait_for_pods; then
        echo_warning "Some pods may not be ready. Continuing..."
    fi

    # Create NodePort services
    if ! create_nodeport_services; then
        echo_warning "Failed to create NodePort services. You may need to use port-forwarding."
    fi

    # Show deployment status
    show_status

    # Run health checks
    echo_info "Waiting 30 seconds for services to stabilize before running health checks..."
    sleep 30
    run_health_checks

    echo ""
    echo_success "ROS-OCP deployment completed!"
    echo_info "The services are now running in KIND cluster '$KIND_CLUSTER_NAME'"
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
    "health")
        run_health_checks
        exit $?
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  (none)          - Deploy ROS-OCP to KIND cluster"
        echo "  cleanup         - Delete Helm release and namespace"
        echo "  cleanup --all   - Delete entire KIND cluster"
        echo "  status          - Show deployment status"
        echo "  health          - Run health checks"
        echo "  help            - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  KIND_CLUSTER_NAME - Name of KIND cluster (default: ros-ocp-cluster)"
        echo "  HELM_RELEASE_NAME - Name of Helm release (default: ros-ocp)"
        echo "  NAMESPACE         - Kubernetes namespace (default: ros-ocp)"
        echo "  STORAGE_CLASS     - Storage class name (default: standard)"
        echo "  CONTAINER_RUNTIME - Container runtime: auto, docker, podman (default: auto)"
        echo ""
        echo "Requirements:"
        echo "  - Docker or Podman must be running (auto-detected by default)"
        echo "  - kubectl, helm, and kind must be installed"
        exit 0
        ;;
esac

# Run main function
main "$@"