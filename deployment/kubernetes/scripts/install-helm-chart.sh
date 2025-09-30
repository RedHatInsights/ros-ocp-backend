#!/bin/bash

# ROS-OCP Helm Chart Installation Script
# This script deploys the ROS-OCP Helm chart to a Kubernetes cluster
# By default, it downloads and uses the latest release from GitHub
# Set USE_LOCAL_CHART=true to use local chart source instead
# Requires: kubectl configured with target cluster context, helm installed, curl, jq

set -e  # Exit on any error

# Trap to cleanup downloaded charts on script exit
trap 'cleanup_downloaded_chart' EXIT INT TERM

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-ros-ocp}
NAMESPACE=${NAMESPACE:-ros-ocp}
VALUES_FILE=${VALUES_FILE:-}
REPO_OWNER="insights-onprem"
REPO_NAME="ros-helm-chart"
USE_LOCAL_CHART=${USE_LOCAL_CHART:-false}  # Set to true to use local chart instead of GitHub release
LOCAL_CHART_PATH=${LOCAL_CHART_PATH:-../helm/ros-ocp}  # Path to local chart directory

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

# Function to check prerequisites for Helm installation
check_prerequisites() {
    echo_info "Checking prerequisites for Helm chart installation..."

    local missing_tools=()

    if ! command_exists kubectl; then
        missing_tools+=("kubectl")
    fi

    if ! command_exists helm; then
        missing_tools+=("helm")
    fi

    if ! command_exists jq; then
        missing_tools+=("jq")
    fi

    if [ ${#missing_tools[@]} -gt 0 ]; then
        echo_error "Missing required tools: ${missing_tools[*]}"
        echo_info "Please install the missing tools:"

        for tool in "${missing_tools[@]}"; do
            case $tool in
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
                "jq")
                    echo_info "  Install jq: https://stedolan.github.io/jq/download/"
                    if [[ "$OSTYPE" == "darwin"* ]]; then
                        echo_info "  macOS: brew install jq"
                    fi
                    ;;
            esac
        done

        return 1
    fi

    # Check kubectl context
    echo_info "Checking kubectl context..."
    local current_context=$(kubectl config current-context 2>/dev/null || echo "none")
    if [ "$current_context" = "none" ]; then
        echo_error "No kubectl context is set. Please configure kubectl to connect to your cluster."
        echo_info "For KIND cluster: kubectl config use-context kind-ros-ocp-cluster"
        echo_info "For OpenShift: oc login <cluster-url>"
        return 1
    fi

    echo_info "Current kubectl context: $current_context"

    # Test kubectl connectivity
    if ! kubectl get nodes >/dev/null 2>&1; then
        echo_error "Cannot connect to cluster. Please check your kubectl configuration."
        return 1
    fi

    echo_success "All prerequisites are met"
    return 0
}

# Function to detect platform (Kubernetes vs OpenShift)
detect_platform() {
    echo_info "Detecting platform..."

    if kubectl get routes.route.openshift.io >/dev/null 2>&1; then
        echo_success "Detected OpenShift platform"
        export PLATFORM="openshift"
        # Use OpenShift values if available and no custom values specified
        if [ -z "$VALUES_FILE" ] && [ -f "$SCRIPT_DIR/../../../openshift-values.yaml" ]; then
            echo_info "Using OpenShift-specific values file"
            VALUES_FILE="$SCRIPT_DIR/../../../openshift-values.yaml"
        fi
    else
        echo_success "Detected Kubernetes platform"
        export PLATFORM="kubernetes"
    fi
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

# Function to download latest chart from GitHub
download_latest_chart() {
    echo_info "Downloading latest Helm chart from GitHub..."

    # Create temporary directory for chart download
    local temp_dir=$(mktemp -d)
    local chart_path=""

    # Get the latest release info from GitHub API
    echo_info "Fetching latest release information from GitHub..."
    local latest_release
    if ! latest_release=$(curl -s "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"); then
        echo_error "Failed to fetch release information from GitHub API"
        rm -rf "$temp_dir"
        return 1
    fi

    # Extract the tag name and download URL for the .tgz file
    local tag_name=$(echo "$latest_release" | jq -r '.tag_name')
    local download_url=$(echo "$latest_release" | jq -r '.assets[] | select(.name | endswith(".tgz")) | .browser_download_url')
    local filename=$(echo "$latest_release" | jq -r '.assets[] | select(.name | endswith(".tgz")) | .name')

    if [ -z "$download_url" ] || [ "$download_url" = "null" ]; then
        echo_error "No .tgz file found in the latest release ($tag_name)"
        echo_info "Available assets:"
        echo "$latest_release" | jq -r '.assets[].name' | sed 's/^/  - /'
        rm -rf "$temp_dir"
        return 1
    fi

    echo_info "Latest release: $tag_name"
    echo_info "Downloading: $filename"
    echo_info "From: $download_url"

    # Download the chart
    if ! curl -L -o "$temp_dir/$filename" "$download_url"; then
        echo_error "Failed to download chart from GitHub"
        rm -rf "$temp_dir"
        return 1
    fi

    # Verify the download
    if [ ! -f "$temp_dir/$filename" ]; then
        echo_error "Downloaded chart file not found: $temp_dir/$filename"
        rm -rf "$temp_dir"
        return 1
    fi

    local file_size=$(stat -c%s "$temp_dir/$filename" 2>/dev/null || stat -f%z "$temp_dir/$filename" 2>/dev/null)
    echo_success "Downloaded chart: $filename (${file_size} bytes)"

    # Export the chart path for use by deploy_helm_chart function
    export DOWNLOADED_CHART_PATH="$temp_dir/$filename"
    export CHART_TEMP_DIR="$temp_dir"

    return 0
}

# Function to cleanup downloaded chart
cleanup_downloaded_chart() {
    if [ -n "$CHART_TEMP_DIR" ] && [ -d "$CHART_TEMP_DIR" ]; then
        echo_info "Cleaning up downloaded chart..."
        rm -rf "$CHART_TEMP_DIR"
        unset DOWNLOADED_CHART_PATH
        unset CHART_TEMP_DIR
    fi
}

# Function to check for Kafka cluster ID conflicts
check_kafka_cluster_conflicts() {
    echo_info "Checking for potential Kafka cluster ID conflicts..."

    # Check if Kafka and Zookeeper PVCs exist with different ages
    local kafka_pvc_age=$(kubectl get pvc -n "$NAMESPACE" -o json 2>/dev/null | jq -r '.items[] | select(.metadata.name | contains("kafka-storage")) | .metadata.creationTimestamp' | head -1)
    local zk_pvc_age=$(kubectl get pvc -n "$NAMESPACE" -o json 2>/dev/null | jq -r '.items[] | select(.metadata.name | contains("zookeeper-storage")) | .metadata.creationTimestamp' | head -1)

    if [ -n "$kafka_pvc_age" ] && [ -n "$zk_pvc_age" ]; then
        # Convert timestamps to seconds for comparison
        local kafka_seconds=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$kafka_pvc_age" "+%s" 2>/dev/null || date -d "$kafka_pvc_age" "+%s" 2>/dev/null)
        local zk_seconds=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$zk_pvc_age" "+%s" 2>/dev/null || date -d "$zk_pvc_age" "+%s" 2>/dev/null)

        if [ -n "$kafka_seconds" ] && [ -n "$zk_seconds" ]; then
            local age_diff=$((kafka_seconds - zk_seconds))
            local age_diff_abs=${age_diff#-}  # absolute value

            # If age difference is more than 1 hour (3600 seconds), likely a conflict
            if [ "$age_diff_abs" -gt 3600 ]; then
                echo_warning "Detected potential Kafka cluster ID conflict:"
                echo_warning "  Kafka PVC age: $kafka_pvc_age"
                echo_warning "  Zookeeper PVC age: $zk_pvc_age"
                echo_warning "  Age difference: $age_diff_abs seconds"
                return 1
            fi
        fi
    fi

    echo_success "No Kafka cluster ID conflicts detected"
    return 0
}

# Function to clean up conflicting Kafka/Zookeeper data
cleanup_kafka_conflicts() {
    echo_warning "Cleaning up Kafka cluster ID conflicts..."

    # Scale down Kafka and Zookeeper
    echo_info "Scaling down Kafka and Zookeeper..."
    kubectl scale statefulset "$HELM_RELEASE_NAME-kafka" --replicas=0 -n "$NAMESPACE" 2>/dev/null || true
    kubectl scale statefulset "$HELM_RELEASE_NAME-zookeeper" --replicas=0 -n "$NAMESPACE" 2>/dev/null || true

    # Wait for pods to terminate
    echo_info "Waiting for Kafka and Zookeeper pods to terminate..."
    local timeout=60
    local count=0
    while [ $count -lt $timeout ]; do
        local kafka_pods=$(kubectl get pods -n "$NAMESPACE" --no-headers | grep -E "(kafka|zookeeper)" | wc -l)
        if [ "$kafka_pods" -eq 0 ]; then
            break
        fi
        sleep 2
        count=$((count + 2))
    done

    # Delete conflicting PVCs
    echo_info "Removing conflicting persistent volumes..."
    kubectl delete pvc -n "$NAMESPACE" -l "app.kubernetes.io/name=kafka" 2>/dev/null || true
    kubectl delete pvc -n "$NAMESPACE" -l "app.kubernetes.io/name=zookeeper" 2>/dev/null || true
    kubectl delete pvc -n "$NAMESPACE" --selector="app.kubernetes.io/instance=$HELM_RELEASE_NAME" --field-selector="metadata.name~=kafka-storage" 2>/dev/null || true
    kubectl delete pvc -n "$NAMESPACE" --selector="app.kubernetes.io/instance=$HELM_RELEASE_NAME" --field-selector="metadata.name~=zookeeper-storage" 2>/dev/null || true

    # Alternative method - delete by name pattern
    kubectl get pvc -n "$NAMESPACE" -o name 2>/dev/null | grep -E "(kafka|zookeeper)" | xargs -r kubectl delete -n "$NAMESPACE" 2>/dev/null || true

    echo_success "Kafka cluster conflict cleanup completed"
}

# Function to verify and create required Kafka topics
verify_kafka_topics() {
    echo_info "Verifying Kafka topics after deployment..."

    # Wait for Kafka to be ready
    echo_info "Waiting for Kafka to be ready..."
    kubectl wait --for=condition=ready pod -l "app.kubernetes.io/name=kafka" \
        --namespace "$NAMESPACE" \
        --timeout=300s || {
        echo_warning "Kafka pod not ready within timeout, will retry topic creation later"
        return 0
    }

    # Get Kafka pod name
    local kafka_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=kafka" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -z "$kafka_pod" ]; then
        echo_warning "No Kafka pod found, skipping topic verification"
        return 0
    fi

    echo_info "Found Kafka pod: $kafka_pod"

    # Required topics
    local required_topics=(
        "hccm.ros.events"
        "rosocp.kruize.recommendations"
        "platform.upload.announce"
        "platform.payload-status"
    )

    # Check and create missing topics
    for topic in "${required_topics[@]}"; do
        echo_info "Checking topic: $topic"
        if ! kubectl exec "$kafka_pod" -n "$NAMESPACE" -c kafka -- kafka-topics --bootstrap-server localhost:29092 --list 2>/dev/null | grep -q "^${topic}$"; then
            echo_info "Creating missing topic: $topic"
            kubectl exec "$kafka_pod" -n "$NAMESPACE" -c kafka -- kafka-topics \
                --bootstrap-server localhost:29092 \
                --create \
                --topic "$topic" \
                --partitions 3 \
                --replication-factor 1 \
                --if-not-exists 2>/dev/null || {
                echo_warning "Failed to create topic: $topic"
            }
        else
            echo_success "Topic exists: $topic"
        fi
    done

    echo_success "Kafka topics verification completed"
}

# Function to deploy Helm chart
deploy_helm_chart() {
    echo_info "Deploying ROS-OCP Helm chart..."

    local chart_source=""

    # Determine chart source
    if [ "$USE_LOCAL_CHART" = "true" ]; then
        echo_info "Using local chart source (USE_LOCAL_CHART=true)"
        cd "$SCRIPT_DIR"

        # Check if Helm chart directory exists
        if [ ! -d "$LOCAL_CHART_PATH" ]; then
            echo_error "Local Helm chart directory not found: $LOCAL_CHART_PATH"
            echo_info "Set USE_LOCAL_CHART=false to use GitHub releases, or set LOCAL_CHART_PATH to the correct chart location (default: ./helm/ros-ocp)"
            return 1
        fi

        chart_source="$LOCAL_CHART_PATH"
        echo_info "Using local chart: $chart_source"
    else
        echo_info "Using GitHub release (USE_LOCAL_CHART=false)"

        # Download latest chart if not already downloaded
        if [ -z "$DOWNLOADED_CHART_PATH" ]; then
            if ! download_latest_chart; then
                echo_error "Failed to download latest chart from GitHub"
                echo_info "Fallback: Set USE_LOCAL_CHART=true to use local chart"
                return 1
            fi
        fi

        chart_source="$DOWNLOADED_CHART_PATH"
        echo_info "Using downloaded chart: $chart_source"
    fi

    # Build Helm command
    local helm_cmd="helm upgrade --install \"$HELM_RELEASE_NAME\" \"$chart_source\""
    helm_cmd="$helm_cmd --namespace \"$NAMESPACE\""
    helm_cmd="$helm_cmd --create-namespace"
    helm_cmd="$helm_cmd --timeout=600s"
    helm_cmd="$helm_cmd --wait"

    # Add values file if specified
    if [ -n "$VALUES_FILE" ]; then
        if [ -f "$VALUES_FILE" ]; then
            echo_info "Using values file: $VALUES_FILE"
            helm_cmd="$helm_cmd -f \"$VALUES_FILE\""
        else
            echo_error "Values file not found: $VALUES_FILE"
            return 1
        fi
    fi

    echo_info "Executing: $helm_cmd"

    # Execute Helm command
    eval $helm_cmd

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

    # Wait for all pods to be ready (excluding jobs) with extended timeout for full deployment
    kubectl wait --for=condition=ready pod -l "app.kubernetes.io/instance=$HELM_RELEASE_NAME" \
        --namespace "$NAMESPACE" \
        --timeout=900s \
        --field-selector=status.phase!=Succeeded

    echo_success "All pods are ready"
}

# Function to show deployment status
show_status() {
    echo_info "Deployment Status"
    echo_info "=================="

    echo_info "Platform: $PLATFORM"
    echo_info "Namespace: $NAMESPACE"
    echo_info "Helm Release: $HELM_RELEASE_NAME"
    if [ -n "$VALUES_FILE" ]; then
        echo_info "Values File: $VALUES_FILE"
    fi
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

    # Show access points based on platform
    if [ "$PLATFORM" = "openshift" ]; then
        echo_info "OpenShift Routes:"
        kubectl get routes -n "$NAMESPACE" 2>/dev/null || echo "  No routes found"
        echo ""

        # Get route hosts for access
        local main_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/")].spec.host}' 2>/dev/null)
        local ingress_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/api/ingress")].spec.host}' 2>/dev/null)
        local kruize_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/api/kruize")].spec.host}' 2>/dev/null)

        if [ -n "$main_route" ]; then
            echo_info "Access Points (via OpenShift Routes):"
            echo_info "  - Main API: http://$main_route/status"
            if [ -n "$ingress_route" ]; then
                echo_info "  - Ingress API: http://$ingress_route/ready"
            fi
            if [ -n "$kruize_route" ]; then
                echo_info "  - Kruize API: http://$kruize_route/api/kruize/listPerformanceProfiles"
            fi
        else
            echo_warning "Routes not found. Use port-forwarding or check route configuration."
        fi
    else
        echo_info "Ingress:"
        kubectl get ingress -n "$NAMESPACE" 2>/dev/null || echo "  No ingress found"
        echo ""

        # For Kubernetes/KIND, use hardcoded port from extraPortMappings (KIND-mapped port)
        local http_port="32061"
        local hostname="localhost:$http_port"
        echo_info "Access Points (via Ingress - for KIND):"
        echo_info "  - Ingress API: http://$hostname/ready"
        echo_info "  - ROS-OCP API: http://$hostname/status"
        echo_info "  - Kruize API: http://$hostname/api/kruize/listPerformanceProfiles"
        echo_info "  - MinIO Console: http://$hostname/minio (minioaccesskey/miniosecretkey)"
    fi
    echo ""

    echo_info "Useful Commands:"
    echo_info "  - View logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/instance=$HELM_RELEASE_NAME"
    echo_info "  - Delete deployment: kubectl delete namespace $NAMESPACE"
    echo_info "  - Run tests: ./test-k8s-dataflow.sh"
}

# Function to check ingress controller readiness
check_ingress_readiness() {
    echo_info "Checking ingress controller readiness before health checks..."

    # Check if we're on Kubernetes (not OpenShift)
    if [ "$PLATFORM" != "kubernetes" ]; then
        echo_info "Skipping ingress readiness check for OpenShift platform"
        return 0
    fi

    # Check if ingress controller pod is running and ready
    local pod_status=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
    local pod_ready=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)

    if [ "$pod_status" != "Running" ] || [ "$pod_ready" != "True" ]; then
        echo_warning "Ingress controller pod not ready (status: $pod_status, ready: $pod_ready)"
        echo_info "Waiting for ingress controller to be ready..."

        # Wait for pod to be ready
        kubectl wait --namespace ingress-nginx \
            --for=condition=ready pod \
            --selector=app.kubernetes.io/name=ingress-nginx \
            --timeout=300s
    fi

    # Get pod name for log checks
    local pod_name=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$pod_name" ]; then
        echo_info "Checking ingress controller logs for readiness indicators..."
        local log_output=$(kubectl logs -n ingress-nginx "$pod_name" --tail=20 2>/dev/null)

        # Look for key readiness indicators in logs
        if echo "$log_output" | grep -q "Starting NGINX Ingress controller" && \
           echo "$log_output" | grep -q "Configuration changes detected"; then
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

    # Test actual connectivity to the ingress controller
    # Use hardcoded port from extraPortMappings (KIND-mapped port)
    local http_port="32061"
    echo_info "Testing connectivity to ingress controller on port $http_port..."
    local connectivity_ok=false
    for i in {1..10}; do
        if curl -f -s "http://localhost:$http_port/ready" >/dev/null 2>&1; then
            echo_success "✓ Ingress controller is accessible via HTTP"
            connectivity_ok=true
            break
        fi
        echo_info "Testing connectivity... ($i/10)"
        sleep 3
    done

    if [ "$connectivity_ok" = false ]; then
        echo_error "✗ Ingress controller is NOT accessible via HTTP despite readiness checks passing"
        echo_error "This indicates a deeper networking issue. Running diagnostics..."

        # Enhanced diagnostics for connectivity failures
        echo_info "=== DIAGNOSTICS: Ingress Controller Connectivity Issue ==="

        # Check service details
        echo_info "Service details:"
        kubectl get service ingress-nginx-controller -n ingress-nginx -o yaml | grep -A 10 -B 5 "nodePort\|type\|ports"

        # Check endpoints
        echo_info "Service endpoints:"
        kubectl get endpoints ingress-nginx-controller -n ingress-nginx -o wide

        # Check if the port is actually listening on the host
        echo_info "Checking if port $http_port is listening on localhost..."
        if command -v netstat >/dev/null 2>&1; then
            netstat -tlnp | grep ":$http_port " || echo "Port $http_port not found in netstat output"
        elif command -v ss >/dev/null 2>&1; then
            ss -tlnp | grep ":$http_port " || echo "Port $http_port not found in ss output"
        fi

        # Check KIND cluster port mapping
        echo_info "Checking KIND cluster port mapping..."
        # Use environment variable for container runtime (defaults to podman)
        local container_runtime="${CONTAINER_RUNTIME:-podman}"

        if command -v "$container_runtime" >/dev/null 2>&1; then
            echo "${container_runtime^} port mapping for KIND cluster:"
            local kind_mappings=$($container_runtime port "${KIND_CLUSTER_NAME:-kind}-control-plane" 2>/dev/null)
            echo "$kind_mappings"

            if echo "$kind_mappings" | grep -q "$http_port"; then
                echo_info "✓ Port $http_port is mapped in KIND cluster"
            else
                echo_error "✗ Port $http_port is NOT mapped in KIND cluster"
                echo_error "This is likely the root cause of the connectivity issue"
                echo_info "Expected mapping should be: 0.0.0.0:$http_port->80/tcp"
            fi
        else
            echo_warning "Container runtime '$container_runtime' not found for port mapping check"
            echo_info "Set CONTAINER_RUNTIME environment variable (e.g., 'docker' or 'podman')"
        fi

        # Test with verbose curl and check if requests reach the controller
        echo_info "Testing with verbose curl to see detailed error:"
        curl -v "http://localhost:$http_port/ready" 2>&1 | head -20 || true

        # Check if the request reached the ingress controller by examining logs
        echo_info "Checking ingress controller logs for incoming requests..."
        echo_info "Looking for request logs in the last 30 seconds..."

        # Get current timestamp for log filtering
        local current_time=$(date +%s)
        local log_start_time=$((current_time - 30))

        # Check logs for HTTP requests
        local request_logs=$(kubectl logs -n ingress-nginx "$pod_name" --since=30s 2>/dev/null | grep -E "(GET|POST|PUT|DELETE|HEAD)" || echo "No HTTP request logs found")

        if [ -n "$request_logs" ] && [ "$request_logs" != "No HTTP request logs found" ]; then
            echo_info "✓ Found HTTP request logs in ingress controller:"
            echo "$request_logs" | head -10
        else
            echo_warning "⚠ No HTTP request logs found in ingress controller"
            echo_warning "This suggests requests are not reaching the controller"

            # Check if there are any access logs at all
            echo_info "Checking for any access logs in the last 5 minutes..."
            local all_logs=$(kubectl logs -n ingress-nginx "$pod_name" --since=5m 2>/dev/null | grep -i "access\|request\|GET\|POST" || echo "No access logs found")
            if [ -n "$all_logs" ] && [ "$all_logs" != "No access logs found" ]; then
                echo_info "Found some access logs:"
                echo "$all_logs" | tail -5
            else
                echo_warning "No access logs found at all - controller may not be processing any requests"
            fi
        fi

        # Check if there are any network policies blocking traffic
        echo_info "Checking for network policies that might block traffic:"
        kubectl get networkpolicies -A 2>/dev/null || echo "No network policies found"

        # Check ingress controller logs for any errors
        echo_info "Checking ingress controller logs for errors:"
        kubectl logs -n ingress-nginx "$pod_name" --tail=50 | grep -i error || echo "No obvious errors in recent logs"

        echo_error "=== END DIAGNOSTICS ==="
        echo_warning "This may cause health checks to fail, but deployment will continue"
    fi

    echo_info "Ingress readiness check completed"
}

# Function to run health checks
run_health_checks() {
    echo_info "Running health checks..."

    local failed_checks=0

    if [ "$PLATFORM" = "openshift" ]; then
        # For OpenShift, test internal connectivity first (this should always work)
        echo_info "Testing internal service connectivity..."

        # Test ROS-OCP API internally
        local api_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=rosocp-api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$api_pod" ]; then
            if kubectl exec -n "$NAMESPACE" "$api_pod" -- curl -f -s http://localhost:8000/status >/dev/null 2>&1; then
                echo_success "✓ ROS-OCP API service is healthy (internal)"
            else
                echo_error "✗ ROS-OCP API service is not responding (internal)"
                failed_checks=$((failed_checks + 1))
            fi
        else
            echo_error "✗ ROS-OCP API pod not found"
            failed_checks=$((failed_checks + 1))
        fi

        # Test Ingress internally via service endpoint
        echo_info "Testing Ingress API via service endpoint..."
        if kubectl run curl-test --image=curlimages/curl:latest --rm -i --restart=Never -n "$NAMESPACE" -- curl -f -s http://ros-ocp-ingress:8080/ready >/dev/null 2>&1; then
            echo_success "✓ Ingress API service is healthy (internal)"
        else
            echo_error "✗ Ingress API service is not responding (internal)"
            failed_checks=$((failed_checks + 1))
        fi

        # Test Kruize internally via service endpoint
        echo_info "Testing Kruize API via service endpoint..."
        if kubectl run curl-test-kruize --image=curlimages/curl:latest --rm -i --restart=Never -n "$NAMESPACE" -- curl -f -s http://ros-ocp-kruize:8080/listPerformanceProfiles >/dev/null 2>&1; then
            echo_success "✓ Kruize API service is healthy (internal)"
        else
            echo_error "✗ Kruize API service is not responding (internal)"
            failed_checks=$((failed_checks + 1))
        fi

        # Test external route accessibility (informational only - not counted as failure)
        echo_info "Testing external route accessibility (informational)..."
        local main_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/")].spec.host}' 2>/dev/null)
        local ingress_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/api/ingress")].spec.host}' 2>/dev/null)
        local kruize_route=$(kubectl get route -n "$NAMESPACE" -o jsonpath='{.items[?(@.spec.path=="/api/kruize")].spec.host}' 2>/dev/null)

        local external_accessible=0

        if [ -n "$main_route" ] && curl -f -s "http://$main_route/status" >/dev/null 2>&1; then
            echo_success "  → ROS-OCP API externally accessible: http://$main_route/status"
            external_accessible=$((external_accessible + 1))
        fi

        if [ -n "$ingress_route" ] && curl -f -s "http://$ingress_route/ready" >/dev/null 2>&1; then
            echo_success "  → Ingress API externally accessible: http://$ingress_route/ready"
            external_accessible=$((external_accessible + 1))
        fi

        if [ -n "$kruize_route" ] && curl -f -s "http://$kruize_route/api/kruize/listPerformanceProfiles" >/dev/null 2>&1; then
            echo_success "  → Kruize API externally accessible: http://$kruize_route/api/kruize/listPerformanceProfiles"
            external_accessible=$((external_accessible + 1))
        fi

        if [ $external_accessible -eq 0 ]; then
            echo_info "  → External routes not accessible (common in internal/corporate clusters)"
            echo_info "  → Use port-forwarding: kubectl port-forward svc/ros-ocp-rosocp-api -n $NAMESPACE 8001:8000"
        else
            echo_success "  → $external_accessible route(s) externally accessible"
        fi

    else
        # For Kubernetes/KIND, use hardcoded port from extraPortMappings (KIND-mapped port)
        echo_info "Using hardcoded ingress HTTP port for KIND cluster..."
        local http_port="32061"
        local hostname="localhost:$http_port"
        echo_info "Using ingress HTTP port: $http_port"
        echo_info "Testing connectivity to http://$hostname..."

        # Check if ingress is accessible
        echo_info "Testing Ingress API: http://$hostname/ready"
        if curl -f -s "http://$hostname/ready" >/dev/null; then
            echo_success "✓ Ingress API is accessible via http://$hostname/ready"
        else
            echo_error "✗ Ingress API is not accessible via http://$hostname/ready"
            echo_info "Debug: Testing root endpoint first..."
            curl -v "http://$hostname/" || echo "Root endpoint also failed"
            failed_checks=$((failed_checks + 1))
        fi

        # Check if ROS-OCP API is accessible via Ingress
        echo_info "Testing ROS-OCP API: http://$hostname/status"
        if curl -f -s "http://$hostname/status" >/dev/null; then
            echo_success "✓ ROS-OCP API is accessible via http://$hostname/status"
        else
            echo_error "✗ ROS-OCP API is not accessible via http://$hostname/status"
            failed_checks=$((failed_checks + 1))
        fi

        # Check if Kruize is accessible
        echo_info "Testing Kruize API: http://$hostname/api/kruize/listPerformanceProfiles"
        if curl -f -s "http://$hostname/api/kruize/listPerformanceProfiles" >/dev/null; then
            echo_success "✓ Kruize API is accessible via http://$hostname/api/kruize/listPerformanceProfiles"
        else
            echo_error "✗ Kruize API is not accessible via http://$hostname/api/kruize/listPerformanceProfiles"
            failed_checks=$((failed_checks + 1))
        fi

        # Check if MinIO console is accessible via ingress
        echo_info "Testing MinIO console: http://$hostname/minio/"
        if curl -f -s "http://$hostname/minio/" >/dev/null; then
            echo_success "✓ MinIO console is accessible via http://$hostname/minio/"
        else
            echo_error "✗ MinIO console is not accessible via http://$hostname/minio/"
            failed_checks=$((failed_checks + 1))
        fi
    fi

    if [ $failed_checks -eq 0 ]; then
        echo_success "All core services are healthy and operational!"
    else
        echo_error "$failed_checks core service check(s) failed"
        echo_info "Check pod logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/instance=$HELM_RELEASE_NAME"
    fi

    return $failed_checks
}

# Function to cleanup
cleanup() {
    local complete_cleanup=false

    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --kafka-conflicts)
                echo_info "Cleaning up Kafka cluster ID conflicts only..."
                if ! check_kafka_cluster_conflicts; then
                    cleanup_kafka_conflicts
                else
                    echo_info "No Kafka conflicts detected"
                fi
                return 0
                ;;
            --complete)
                complete_cleanup=true
                ;;
            *)
                echo_warning "Unknown cleanup option: $1"
                ;;
        esac
        shift
    done

    echo_info "Cleaning up Helm deployment..."

    # Check if namespace exists
    if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
        echo_info "Namespace '$NAMESPACE' does not exist"
        return 0
    fi

    # Delete Helm release first
    echo_info "Deleting Helm release..."
    if helm list -n "$NAMESPACE" | grep -q "$HELM_RELEASE_NAME"; then
        helm uninstall "$HELM_RELEASE_NAME" -n "$NAMESPACE" || true
        echo_info "Waiting for Helm release deletion to complete..."
        sleep 5
    else
        echo_info "Helm release '$HELM_RELEASE_NAME' not found"
    fi

    # Delete PVCs explicitly (they often persist after namespace deletion)
    echo_info "Deleting Persistent Volume Claims..."
    local pvcs=$(kubectl get pvc -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)
    if [ -n "$pvcs" ]; then
        for pvc in $pvcs; do
            echo_info "Deleting PVC: $pvc"
            kubectl delete pvc "$pvc" -n "$NAMESPACE" --timeout=60s || true
        done

        # Wait for PVCs to be fully deleted
        echo_info "Waiting for PVCs to be deleted..."
        local timeout=60
        local count=0
        while [ $count -lt $timeout ]; do
            local remaining_pvcs=$(kubectl get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
            if [ "$remaining_pvcs" -eq 0 ]; then
                echo_success "All PVCs deleted"
                break
            fi
            echo_info "Waiting for $remaining_pvcs PVCs to be deleted... ($count/$timeout seconds)"
            sleep 2
            count=$((count + 2))
        done

        if [ $count -ge $timeout ]; then
            echo_warning "Timeout waiting for PVCs to be deleted. Some may still exist."
        fi
    else
        echo_info "No PVCs found in namespace"
    fi

    # Complete cleanup includes orphaned PVs
    if [ "$complete_cleanup" = true ]; then
        echo_info "Performing complete cleanup including orphaned Persistent Volumes..."
        local orphaned_pvs=$(kubectl get pv -o jsonpath='{.items[?(@.spec.claimRef.namespace=="'$NAMESPACE'")].metadata.name}' 2>/dev/null || true)
        if [ -n "$orphaned_pvs" ]; then
            for pv in $orphaned_pvs; do
                echo_info "Deleting orphaned PV: $pv"
                kubectl delete pv "$pv" --timeout=30s || true
            done
        else
            echo_info "No orphaned PVs found"
        fi
    fi

    # Delete namespace
    echo_info "Deleting namespace..."
    kubectl delete namespace "$NAMESPACE" --timeout=120s || true

    # Wait for namespace deletion
    echo_info "Waiting for namespace deletion to complete..."
    local timeout=120
    local count=0
    while [ $count -lt $timeout ]; do
        if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
            echo_success "Namespace deleted successfully"
            break
        fi
        echo_info "Waiting for namespace deletion... ($count/$timeout seconds)"
        sleep 2
        count=$((count + 2))
    done

    if [ $count -ge $timeout ]; then
        echo_warning "Timeout waiting for namespace deletion. It may still be terminating."
    fi

    echo_success "Cleanup completed"

    # Cleanup any downloaded charts
    cleanup_downloaded_chart
}

# Main execution
main() {
    echo_info "ROS-OCP Helm Chart Installation"
    echo_info "==============================="

    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi

    # Detect platform
    detect_platform

    echo_info "Configuration:"
    echo_info "  Platform: $PLATFORM"
    echo_info "  Helm Release: $HELM_RELEASE_NAME"
    echo_info "  Namespace: $NAMESPACE"
    if [ -n "$VALUES_FILE" ]; then
        echo_info "  Values File: $VALUES_FILE"
    fi
    echo ""

    # Create namespace
    if ! create_namespace; then
        exit 1
    fi

    # Check for Kafka cluster ID conflicts
    if ! check_kafka_cluster_conflicts; then
        echo_warning "Kafka cluster ID conflicts detected. Cleaning up..."
        cleanup_kafka_conflicts
    fi

    # Deploy Helm chart
    if ! deploy_helm_chart; then
        exit 1
    fi

    # Verify and create Kafka topics
    echo_info "Post-deployment: Verifying Kafka setup..."
    verify_kafka_topics

    # Wait for pods to be ready
    if ! wait_for_pods; then
        echo_warning "Some pods may not be ready. Continuing..."
    fi

    # Show deployment status
    show_status

    # Check ingress readiness before health checks
    check_ingress_readiness

    # Run health checks
    echo_info "Waiting 30 seconds for services to stabilize before running health checks..."
    sleep 30

    # Show pod status before health checks
    echo_info "Pod status before health checks:"
    kubectl get pods -n "$NAMESPACE" -o wide

    if ! run_health_checks; then
        echo_warning "Some health checks failed, but deployment completed successfully"
        echo_info "Services may need more time to be fully ready"
        echo_info "You can run health checks manually later or check pod logs for issues"
        echo_info "Pod logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/instance=$HELM_RELEASE_NAME"
    fi

    echo ""
    echo_success "ROS-OCP Helm chart installation completed!"
    echo_info "The services are now running in namespace '$NAMESPACE'"

    if [ "$PLATFORM" = "kubernetes" ]; then
        echo_info "Next: Run ./test-k8s-dataflow.sh to test the deployment"
    else
        echo_info "Next: Run ./test-ocp-dataflow.sh to test the deployment"
    fi

    # Cleanup downloaded chart if we used GitHub release
    if [ "$USE_LOCAL_CHART" != "true" ]; then
        cleanup_downloaded_chart
    fi
}

# Handle script arguments
case "${1:-}" in
    "cleanup")
        cleanup
        exit 0
        ;;
    "status")
        detect_platform
        show_status
        exit 0
        ;;
    "health")
        detect_platform
        run_health_checks
        exit $?
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  (none)              - Install ROS-OCP Helm chart"
        echo "  cleanup             - Delete Helm release and namespace (preserves PVs)"
        echo "  cleanup --complete  - Complete removal including Persistent Volumes"
        echo "  cleanup --kafka-conflicts - Clean up Kafka cluster ID conflicts only"
        echo "  status              - Show deployment status"
        echo "  health              - Run health checks"
        echo "  help                - Show this help message"
        echo ""
        echo "Uninstall/Reinstall Workflow:"
        echo "  # For clean reinstall with fresh data:"
        echo "  $0 cleanup --complete    # Remove everything including data"
        echo "  $0 install               # Fresh installation"
        echo ""
        echo "  # For reinstall preserving data:"
        echo "  $0 cleanup               # Remove workloads but keep volumes"
        echo "  $0 install               # Reinstall (reuses existing volumes)"
        echo ""
        echo "Environment Variables:"
        echo "  HELM_RELEASE_NAME - Name of Helm release (default: ros-ocp)"
        echo "  NAMESPACE         - Kubernetes namespace (default: ros-ocp)"
        echo "  VALUES_FILE       - Path to custom values file (optional)"
        echo "  USE_LOCAL_CHART   - Use local chart instead of GitHub release (default: false)"
        echo "  LOCAL_CHART_PATH  - Path to local chart directory (default: ../helm/ros-ocp)"
        echo ""
        echo "Chart Source Options:"
        echo "  - Default: Downloads latest release from GitHub (recommended)"
        echo "  - Local: Set USE_LOCAL_CHART=true to use local chart directory"
        echo "  - Chart Path: Set LOCAL_CHART_PATH to specify custom chart location"
        echo "  - Examples:"
        echo "    USE_LOCAL_CHART=true LOCAL_CHART_PATH=../helm/ros-ocp $0"
        echo "    USE_LOCAL_CHART=true LOCAL_CHART_PATH=../ros-helm-chart/ros-ocp $0"
        echo ""
        echo "Platform Detection:"
        echo "  - Automatically detects Kubernetes vs OpenShift"
        echo "  - Uses openshift-values.yaml for OpenShift if available"
        echo "  - Auto-detects optimal storage class for platform"
        echo "  - Detects and resolves Kafka cluster ID conflicts automatically"
        echo "  - Ensures required Kafka topics are created"
        echo ""
        echo "Requirements:"
        echo "  - kubectl must be configured with target cluster"
        echo "  - helm must be installed"
        echo "  - jq must be installed for JSON processing"
        echo "  - Target cluster must have sufficient resources"
        exit 0
        ;;
esac

# Run main function
main "$@"
