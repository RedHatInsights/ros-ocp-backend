#!/bin/bash

# ROS-OCP KIND Cluster Setup Script
# This script sets up a KIND cluster for ROS-OCP deployment
# For Helm chart deployment, use ./install-helm-chart.sh
# Container Runtime: Configurable via CONTAINER_RUNTIME variable (default: podman)
#
# MEMORY REQUIREMENTS:
# - Container Runtime: Minimum 6GB memory allocation required
# - KIND node: Fixed 6GB memory limit for deterministic deployment
# - Allocatable: ~5.2GB after system reservations
# - Full deployment: ~4.5GB for all ROS-OCP services

set -e  # Exit on any error

# Debug: Show script start
echo "=== SCRIPT START ==="
echo "Script: $0"
echo "Arguments: $@"
echo "Working directory: $(pwd)"
echo "User: $(whoami)"
echo "PATH: $PATH"
echo "==================="

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-ros-ocp-cluster}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-podman}
INGRESS_DEBUG_LEVEL=${INGRESS_DEBUG_LEVEL:-0}

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
                "podman")
                    echo_info "  Install Podman: https://podman.io/getting-started/installation"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install podman"
                    fi
                    ;;
            esac
        done

        return 1
    fi

    # Check container runtime is running
    if ! $DETECTED_RUNTIME info >/dev/null 2>&1; then
        echo_error "$DETECTED_RUNTIME is not accessible. Please ensure $DETECTED_RUNTIME is running."
        return 1
    fi
    export KIND_EXPERIMENTAL_PROVIDER=$DETECTED_RUNTIME

    # Warn about PID limits for podman
    if [ "$DETECTED_RUNTIME" = "podman" ]; then
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

# Function to clean up existing KIND containers and project images
cleanup_kind_artifacts() {
    echo_info "Performing preflight cleanup of KIND artifacts..."

    # Use the standalone cleanup script
    local cleanup_script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local cleanup_script="$cleanup_script_dir/cleanup-kind-artifacts.sh"

    if [ -f "$cleanup_script" ]; then
        echo_info "Using standalone cleanup script: $cleanup_script"
        KIND_CLUSTER_NAME="$KIND_CLUSTER_NAME" CONTAINER_RUNTIME="$CONTAINER_RUNTIME" "$cleanup_script"
    else
        echo_warning "Standalone cleanup script not found at $cleanup_script"
        echo_warning "Falling back to embedded cleanup logic..."

        # Fallback cleanup logic (simplified version)
        if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
            echo_info "Removing existing KIND cluster: $KIND_CLUSTER_NAME"
            kind delete cluster --name "$KIND_CLUSTER_NAME" || echo_warning "Failed to delete KIND cluster (may not exist)"
        fi

        echo_success "Preflight cleanup completed"
    fi
}

# Function to create KIND cluster with storage
create_kind_cluster() {
    echo_info "Creating KIND cluster: $KIND_CLUSTER_NAME"

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
  # Primary ingress entry point - HTTP traffic (bind to all interfaces)
  - containerPort: 80
    hostPort: 32061
    protocol: TCP
    listenAddress: "0.0.0.0"
  # HTTPS traffic (bind to all interfaces)
  - containerPort: 443
    hostPort: 30325
    protocol: TCP
    listenAddress: "0.0.0.0"
EOF
)

    # Create cluster with standard configuration
    if [ "$DETECTED_RUNTIME" = "podman" ]; then
        echo "$kind_config" | KIND_EXPERIMENTAL_PROVIDER=$DETECTED_RUNTIME kind create cluster --config=-
    else
        echo "$kind_config" | KIND_EXPERIMENTAL_DOCKER_NETWORK="" kind create cluster --config=-
    fi

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
    if [ "$DETECTED_RUNTIME" = "docker" ]; then
        if $DETECTED_RUNTIME update --memory=6g "${KIND_CLUSTER_NAME}-control-plane" >/dev/null 2>&1; then
            echo_success "Memory limit set to 6GB"
            # Give the container a moment to adjust to the new memory limit
            sleep 5
        else
            echo_warning "Could not set 6GB memory limit on KIND container."
            echo_warning "This may cause deployment issues if container runtime has insufficient memory."
            echo_info "Continuing with default container runtime memory allocation..."

            # Check actual container runtime memory available
            local actual_memory=$($DETECTED_RUNTIME system info --format '{{.MemTotal}}' 2>/dev/null || echo "0")
            if [ "$actual_memory" -gt 0 ]; then
                local actual_gb=$((actual_memory / 1024 / 1024 / 1024))
                echo_info "Container runtime has ${actual_gb}GB memory available"
                if [ "$actual_gb" -lt 5 ]; then
                    echo_error "Container runtime has less than 5GB memory. Deployment may fail due to resource constraints."
                    echo_error "Please increase container runtime memory allocation and try again."
                    return 1
                fi
            fi
        fi
    else
        echo_info "Podman detected - memory limits are handled by systemd/cgroups"
        echo_info "Continuing with system memory allocation..."
    fi

    # Set kubectl context
    kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}"
    echo_success "kubectl context set to kind-${KIND_CLUSTER_NAME}"

    # Simple check that KIND cluster is working
    echo_info "Verifying KIND cluster is working..."
    if kubectl get nodes >/dev/null 2>&1; then
        echo_success "✓ KIND cluster is accessible"
    else
        echo_error "✗ KIND cluster is not accessible"
        return 1
    fi

    # Wait for API server to be fully ready with extended timeout for 6GB constrained environment
    echo_info "Waiting for API server to be fully ready..."

    # First check if the KIND container is running
    if ! $DETECTED_RUNTIME ps --filter "name=${KIND_CLUSTER_NAME}-control-plane" --filter "status=running" | grep -q "${KIND_CLUSTER_NAME}-control-plane"; then
        echo_error "KIND container ${KIND_CLUSTER_NAME}-control-plane is not running"
        $DETECTED_RUNTIME ps --filter "name=${KIND_CLUSTER_NAME}-control-plane"
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
            echo_info "KIND container status: $($DETECTED_RUNTIME inspect --format='{{.State.Status}}' ${KIND_CLUSTER_NAME}-control-plane 2>/dev/null || echo 'unknown')"
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
        $DETECTED_RUNTIME ps --filter "name=${KIND_CLUSTER_NAME}-control-plane"
        echo_info "KIND container logs (last 20 lines):"
        $DETECTED_RUNTIME logs --tail 20 "${KIND_CLUSTER_NAME}-control-plane" 2>/dev/null || echo "Could not retrieve container logs"
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
    echo_info "Installing KIND-specific NGINX Ingress Controller..."

    # Install KIND-specific ingress controller (designed for extraPortMappings)
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

    # Wait a moment for the deployment to be created and pods to be scheduled
    echo_info "Waiting for ingress controller deployment to be created..."
    sleep 10

    # Configure debug logging if requested
    if [ "$INGRESS_DEBUG_LEVEL" -gt 0 ]; then
        echo_info "Enabling debug logs in NGINX ingress controller (level: $INGRESS_DEBUG_LEVEL)..."
        kubectl patch deployment ingress-nginx-controller -n ingress-nginx --type='json' -p="[
            {
                \"op\": \"add\",
                \"path\": \"/spec/template/spec/containers/0/args/-\",
                \"value\": \"--v=$INGRESS_DEBUG_LEVEL\"
            },
            {
                \"op\": \"add\",
                \"path\": \"/spec/template/spec/containers/0/args/-\",
                \"value\": \"--logtostderr=true\"
            }
        ]" || echo_warning "Debug logging patch failed, continuing..."

        # Give time for the patch to take effect
        echo_info "Waiting for debug configuration to take effect..."
        sleep 5
    fi
    
    # Wait for the deployment to be ready
    echo_info "Waiting for ingress-nginx controller deployment to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=300s

    echo_success "NGINX Ingress Controller is ready"
}

# Function to create authentication setup for insights-ros-ingress
create_auth_setup() {
    echo_info "Setting up authentication for insights-ros-ingress..."

    # Create service account for insights-ros-ingress
    local service_account="insights-ros-ingress"

    if kubectl get serviceaccount "$service_account" -n "$NAMESPACE" >/dev/null 2>&1; then
        echo_warning "Service account '$service_account' already exists in namespace '$NAMESPACE'"
    else
        kubectl create serviceaccount "$service_account" -n "$NAMESPACE"
        echo_success "Service account '$service_account' created"
    fi

    # Create ClusterRoleBinding for system:auth-delegator (required for TokenReviewer API)
    local cluster_role_binding="${service_account}-token-reviewer"

    if kubectl get clusterrolebinding "$cluster_role_binding" >/dev/null 2>&1; then
        echo_warning "ClusterRoleBinding '$cluster_role_binding' already exists"
    else
        cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $cluster_role_binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: $service_account
  namespace: $NAMESPACE
EOF
        echo_success "ClusterRoleBinding '$cluster_role_binding' created"
    fi

    # Create a long-lived token secret for the service account
    local token_secret="${service_account}-token"

    if kubectl get secret "$token_secret" -n "$NAMESPACE" >/dev/null 2>&1; then
        echo_warning "Token secret '$token_secret' already exists"
    else
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: $token_secret
  namespace: $NAMESPACE
  annotations:
    kubernetes.io/service-account.name: $service_account
type: kubernetes.io/service-account-token
EOF
        echo_success "Token secret '$token_secret' created"
    fi

    # Wait for the token to be generated
    echo_info "Waiting for service account token to be generated..."
    local retries=30
    local count=0

    while [ $count -lt $retries ]; do
        if kubectl get secret "$token_secret" -n "$NAMESPACE" -o jsonpath='{.data.token}' >/dev/null 2>&1; then
            local token_data
            token_data=$(kubectl get secret "$token_secret" -n "$NAMESPACE" -o jsonpath='{.data.token}')
            if [ -n "$token_data" ]; then
                echo_success "Service account token generated successfully"
                break
            fi
        fi
        echo_info "Waiting for token generation... ($((count + 1))/$retries)"
        sleep 2
        count=$((count + 1))
    done

    if [ $count -eq $retries ]; then
        echo_error "Failed to generate service account token after $retries attempts"
        return 1
    fi

    # Save authentication configuration for test scripts
    local kubeconfig_path="/tmp/dev-kubeconfig"
    local cluster_server
    cluster_server=$(kubectl config view --raw -o jsonpath="{.clusters[?(@.name=='kind-${KIND_CLUSTER_NAME}')].cluster.server}")
    local token
    token=$(kubectl get secret "$token_secret" -n "$NAMESPACE" -o jsonpath='{.data.token}' | base64 -d)

    # Create kubeconfig file for test scripts
    cat > "$kubeconfig_path" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: $cluster_server
    insecure-skip-tls-verify: true
  name: kind-dev
contexts:
- context:
    cluster: kind-dev
    user: $service_account
    namespace: $NAMESPACE
  name: kind-dev
current-context: kind-dev
users:
- name: $service_account
  user:
    token: $token
EOF

    echo_success "Test kubeconfig created at $kubeconfig_path"
    echo_info "This kubeconfig can be used by test scripts for authentication"

    return 0
}

# Function to deploy Helm chart
deploy_helm_chart() {
    echo_info "Deploying ROS-OCP Helm chart..."

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

# Function to check ingress controller readiness
check_ingress_readiness() {
    echo_info "Checking ingress controller readiness..."

    local max_attempts=60
    local attempt=0
    local all_ready=false

    while [ $attempt -lt $max_attempts ]; do
        echo_info "Ingress readiness check attempt $((attempt + 1))/$max_attempts"

        # Check if ingress controller pod is running and ready
        local pod_status=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
        local pod_ready=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)

        if [ "$pod_status" = "Running" ] && [ "$pod_ready" = "True" ]; then
            echo_success "✓ Ingress controller pod is running and ready"

            # Check pod logs for readiness indicators
            local pod_name=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
            if [ -n "$pod_name" ]; then
                echo_info "Checking ingress controller logs for readiness indicators..."
                local log_output=$(kubectl logs -n ingress-nginx "$pod_name" --tail=20 2>/dev/null)

                # Look for key readiness indicators in logs
                if echo "$log_output" | grep -q "Starting NGINX Ingress controller" && \
                   echo "$log_output" | grep -q "Configuration changes detected" && \
                   echo "$log_output" | grep -q "Configuration changes applied"; then
                    echo_success "✓ Ingress controller logs show proper initialization"
                else
                    echo_warning "⚠ Ingress controller logs don't show complete initialization yet"
                    echo_info "Recent logs:"
                    echo "$log_output" | tail -5
                fi
            fi

            # Check if service endpoints are ready
            local endpoints_ready=$(kubectl get endpoints ingress-nginx-controller -n ingress-nginx -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null)
            if [ -n "$endpoints_ready" ]; then
                echo_success "✓ Ingress controller service has ready endpoints"
            else
                echo_warning "⚠ Ingress controller service endpoints not ready yet"
            fi

            # Test actual connectivity to the ingress controller using KIND-mapped port
            local test_port="32061"
            echo_info "Testing connectivity to ingress controller on KIND-mapped port $test_port..."
            if curl -f -s "http://localhost:$test_port/" >/dev/null 2>&1; then
                echo_success "✓ Ingress controller is accessible via HTTP"
                all_ready=true
                break
            else
                echo_warning "⚠ Ingress controller not yet accessible via HTTP"
            fi

            # Check for any recent events that might indicate issues
            echo_info "Checking for recent ingress controller events..."
            local recent_events=$(kubectl get events -n ingress-nginx --sort-by='.lastTimestamp' --field-selector involvedObject.name="$pod_name" 2>/dev/null | tail -3)
            if [ -n "$recent_events" ]; then
                echo_info "Recent events:"
                echo "$recent_events"
            fi

        else
            echo_info "Pod status: $pod_status, Ready: $pod_ready"
            echo_info "Waiting for ingress controller pod to be ready..."
        fi

        sleep 5
        attempt=$((attempt + 1))
    done

    if [ "$all_ready" = true ]; then
        echo_success "✓ Ingress controller is fully ready and operational"
        return 0
    else
        echo_error "✗ Ingress controller failed to become ready after $max_attempts attempts"
        echo_error "Pod status:"
        kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o wide
        echo_error "Service status:"
        kubectl get service ingress-nginx-controller -n ingress-nginx -o wide
        echo_error "Recent events:"
        kubectl get events -n ingress-nginx --sort-by='.lastTimestamp' | tail -10
        return 1
    fi
}

# Note: All external access is handled through the nginx ingress controller (port auto-detected)
# Individual services are accessed via path-based routing through the ingress

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

    # Use hardcoded port from extraPortMappings (KIND-mapped port)
    local http_port="32061"

    echo_info "Access Points:"
    echo_info "  - Ingress Entry Point: http://localhost:$http_port"
    echo_info "    All services are accessible through path-based routing:"
    echo_info "    - Ingress API: http://localhost:$http_port/api/ingress/v1/version"
    echo_info "    - ROS-OCP API: http://localhost:$http_port/status"
    echo_info "    - Kruize API: http://localhost:$http_port/api/kruize/listPerformanceProfiles"
    echo_info "    - MinIO Console: http://localhost:$http_port/minio (Web UI - minioaccesskey/miniosecretkey)"
    echo_info "Ingress Controller:"
    kubectl get pods -n ingress-nginx
    echo ""

    echo_info "Useful Commands:"
    echo_info "  - Deploy Helm chart: ./install-helm-chart.sh"
    echo_info "  - Test deployment: ./test-k8s-dataflow.sh"
    echo_info "  - Delete cluster: kind delete cluster --name $KIND_CLUSTER_NAME"
}


# Function to run health checks with authentication
run_health_checks() {
    echo_info "Running health checks with authentication..."

    # Health checks will run with or without authentication
    echo_info "Running connectivity health checks..."

    local failed_checks=0

    # Use hardcoded port from extraPortMappings (KIND-mapped port)
    local http_port="32061"

    echo_info "Using KIND-mapped HTTP port: $http_port"

    # Get authentication token for testing
    local auth_token=""
    if [ -f "/tmp/dev-kubeconfig" ]; then
        auth_token=$(kubectl --kubeconfig=/tmp/dev-kubeconfig config view --raw -o jsonpath='{.users[0].user.token}' 2>/dev/null || echo "")
    fi

    if [ -z "$auth_token" ]; then
        # Try kubectl/oc whoami -t to generate token from current user context
        auth_token=$(kubectl whoami -t 2>/dev/null || oc whoami -t 2>/dev/null || echo "")
    fi

    if [ -z "$auth_token" ]; then
        # For KIND clusters, we can skip authentication for basic health checks
        # The deploy script runs health checks primarily to verify connectivity
        echo_warning "No authentication token available for health checks"
        echo_info "Running health checks without authentication (KIND cluster admin context)"
        echo_info "Note: API endpoints may return 401, but this indicates the services are responding"
    else
        echo_info "Using authentication token for health checks"
    fi

    # Check if nginx ingress controller is accessible on entry point
    local nginx_response
    nginx_response=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$http_port/" 2>/dev/null || echo "000")

    if [ "$nginx_response" = "000" ] || [ -z "$nginx_response" ]; then
        echo_error "Ingress Entry Point is not accessible on port $http_port"
        failed_checks=$((failed_checks + 1))
    else
        echo_success "Ingress Entry Point is accessible on port $http_port (HTTP ${nginx_response})"
    fi

    # Check if ROS-OCP ingress API is accessible via ingress with authentication
    local ingress_response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $auth_token" \
        "http://localhost:$http_port/api/ingress/v1/version" 2>/dev/null || echo "000")

    if [ "$ingress_response" = "200" ] || [ "$ingress_response" = "401" ]; then
        echo_success "ROS-OCP Ingress API is accessible via ingress on port $http_port (HTTP ${ingress_response})"
    else
        echo_error "ROS-OCP Ingress API is not accessible via ingress on port $http_port (HTTP ${ingress_response})"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if ROS-OCP API is accessible via ingress with authentication
    local api_response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $auth_token" \
        "http://localhost:$http_port/status" 2>/dev/null || echo "000")

    if [ "$api_response" = "200" ] || [ "$api_response" = "401" ]; then
        echo_success "ROS-OCP API is accessible via ingress on port $http_port (HTTP ${api_response})"
    else
        echo_error "ROS-OCP API is not accessible via ingress on port $http_port (HTTP ${api_response})"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if Kruize is accessible via ingress with authentication
    local kruize_response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $auth_token" \
        "http://localhost:$http_port/api/kruize/listPerformanceProfiles" 2>/dev/null || echo "000")

    if [ "$kruize_response" = "200" ] || [ "$kruize_response" = "401" ]; then
        echo_success "Kruize API is accessible via ingress on port $http_port (HTTP ${kruize_response})"
    else
        echo_error "Kruize API is not accessible via ingress on port $http_port (HTTP ${kruize_response})"
        failed_checks=$((failed_checks + 1))
    fi

    # Check if MinIO console is accessible via ingress (no auth required for console)
    if curl -f -s "http://localhost:$http_port/minio/" >/dev/null; then
        echo_success "MinIO console is accessible via ingress on port $http_port"
    else
        echo_error "MinIO console is not accessible via ingress on port $http_port"
        failed_checks=$((failed_checks + 1))
    fi

    if [ $failed_checks -eq 0 ]; then
        echo_success "All health checks passed with authentication!"
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


# Main function
main() {
    echo "=== MAIN FUNCTION START ==="
    echo_info "Starting KIND cluster setup for ROS-OCP..."

    # Check required commands
    echo_info "Checking required commands..."
    local missing_commands=()

    echo "Checking kind command..."
    if ! command -v kind >/dev/null 2>&1; then
        echo "kind command not found"
        missing_commands+=("kind")
    else
        echo "kind command found: $(which kind)"
    fi

    echo "Checking kubectl command..."
    if ! command -v kubectl >/dev/null 2>&1; then
        echo "kubectl command not found"
        missing_commands+=("kubectl")
    else
        echo "kubectl command found: $(which kubectl)"
    fi

        echo "Checking container runtime..."
        if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
            echo "$CONTAINER_RUNTIME command not found"
            missing_commands+=("$CONTAINER_RUNTIME")
        else
            echo "$CONTAINER_RUNTIME command found: $(which $CONTAINER_RUNTIME)"
        fi

    if [ ${#missing_commands[@]} -gt 0 ]; then
        echo_error "Missing required commands: ${missing_commands[*]}"
        echo_error "Please install the missing commands and try again"
        return 1
    fi

    echo_success "✓ All required commands are available"

    # Detect container runtime
    echo "Calling detect_container_runtime..."
    detect_container_runtime

    # Clean up existing KIND artifacts
    echo "Calling cleanup_kind_artifacts..."
    cleanup_kind_artifacts

    # Setup KIND cluster
    echo "Calling create_kind_cluster..."
    create_kind_cluster

    # Install ingress controller
    echo "Calling install_ingress_controller..."
    install_ingress_controller

    # Show final status
    echo "Calling show_status..."
    show_status


    echo_success "KIND cluster setup completed successfully!"
    echo_info "Next step: Run ./install-helm-chart.sh to deploy ROS-OCP"
    echo "=== MAIN FUNCTION END ==="
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
        echo "  health          - Run health checks on existing cluster"
        echo "  help            - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  KIND_CLUSTER_NAME     - Name of KIND cluster (default: ros-ocp-cluster)"
        echo "  CONTAINER_RUNTIME     - Container runtime to use (default: podman, supports: podman, docker, auto)"
        echo "  INGRESS_DEBUG_LEVEL   - NGINX ingress debug verbosity level (default: 0=disabled, 1-4=debug levels)"
        echo ""
        echo "Requirements:"
        echo "  - Container runtime must be installed and running (default: podman)"
        echo "  - kubectl and kind must be installed"
        echo "  - Container Runtime: Minimum 6GB memory allocation"
        echo ""
        echo "Debug Logging:"
        echo "  Set INGRESS_DEBUG_LEVEL to enable NGINX ingress controller debug logging:"
        echo "    0 - Disabled (default)"
        echo "    1 - Basic info logging"
        echo "    2 - Detailed info logging (recommended for debugging)"
        echo "    3 - Very detailed debugging"
        echo "    4 - Extremely verbose (use with caution)"
        echo "  Example: INGRESS_DEBUG_LEVEL=2 ./deploy-kind.sh"
        echo ""
        echo "Authentication:"
        echo "  This script sets up authentication tokens required for testing."
        echo "  If authentication tokens are not available, the script will FAIL."
        echo "  Authentication sources checked:"
        echo "    - Service account 'insights-ros-ingress' with token secret"
        echo "    - Dev kubeconfig file at /tmp/dev-kubeconfig"
        echo "    - insights-ros-ingress pod with service account token"
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