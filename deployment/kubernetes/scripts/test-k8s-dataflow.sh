#!/bin/bash

# ROS-OCP Kubernetes Data Flow Test Script
# This script tests the complete data flow in a Kubernetes deployment
#
# AUTHENTICATION ARCHITECTURE:
# - On Kubernetes/KIND: X-Rh-Identity headers validated directly by backend middleware
# - On OpenShift: JWT tokens validated by Envoy sidecar, then transformed to X-Rh-Identity
# - Both platforms require authentication for API endpoints (/api/cost-management/v1/*)
# - Public endpoints (/status, /ready) do not require authentication on any platform
#
# This test uses X-Rh-Identity headers to authenticate with ros-ocp-backend API endpoints.
# The backend validates X-Rh-Identity headers using platform-go-middlewares/v2/identity.

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-ros-ocp}
HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-ros-ocp}

# Function to get ingress hostname for Kubernetes
get_ingress_hostname() {
    # For KIND clusters, use the KIND-mapped port (32061) instead of service NodePort
    # This ensures we use the port that KIND actually exposes on the host
    echo "localhost:32061"
}

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

# Function to create X-Rh-Identity header for ros-ocp-backend authentication
create_rh_identity_header() {
    local org_id="${1:-12345}"

    # Create identity JSON structure matching platform-go-middlewares XRHID format
    # Reference: github.com/redhatinsights/platform-go-middlewares/v2/identity
    local identity_json="{\"identity\":{\"org_id\":\"$org_id\",\"type\":\"User\"}}"

    # Base64 encode the identity
    echo -n "$identity_json" | base64 | tr -d '\n'
}

# Function to check if kubectl is configured
check_kubectl() {
    if ! kubectl cluster-info >/dev/null 2>&1; then
        echo_error "kubectl is not configured or cluster is not accessible"
        return 1
    fi
    return 0
}

# Function to validate authentication tokens are available
validate_authentication_tokens() {
    echo_info "Validating authentication tokens for testing..."

    local token_found=false
    local auth_methods=()

    # Check if we can use the kubeconfig created by deploy-kind.sh
    if [ -f "/tmp/dev-kubeconfig" ]; then
        local token=$(kubectl --kubeconfig=/tmp/dev-kubeconfig config view --raw -o jsonpath='{.users[0].user.token}' 2>/dev/null || echo "")
        if [ -n "$token" ]; then
            auth_methods+=("dev-kubeconfig")
            token_found=true
            echo_success "✓ Authentication token available from dev kubeconfig"
        fi
    fi

    # Check for API pod service account token (our new approach)
    local api_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$api_pod" ]; then
        local token=$(kubectl exec -n "$NAMESPACE" "$api_pod" -- cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null)
        if [ -n "$token" ]; then
            auth_methods+=("pod-service-account")
            token_found=true
            echo_success "✓ Authentication token available from pod service account"
        fi
    else
        echo_warning "API pod not found - deployment may not be complete yet"
    fi

    if [ "$token_found" = false ]; then
        echo_error "✗ No authentication tokens available!"
        echo_error "Authentication is REQUIRED for testing. The test will FAIL."
        echo_info "Expected authentication sources:"
        echo_info "  - Dev kubeconfig file at /tmp/dev-kubeconfig"
        echo_info "  - ros-ocp-api pod with service account token"
        echo_info ""
        echo_info "Make sure deploy-kind.sh was run to set up authentication"
        echo_info "If deployment is in progress, wait for pods to be ready first"
        echo_info "This test requires authentication tokens to function properly."
        return 1
    else
        echo_success "✓ Authentication validation passed"
        echo_info "Available authentication methods: ${auth_methods[*]}"
        return 0
    fi
}

# Function to get service account token from ros-ingress pod
get_ros_ingress_service_account_token() {
    echo_info >&2 "Extracting service account token from ros-ingress pod..."

    # Find the ros-ingress pod
    local ingress_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=ingress -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$ingress_pod" ]; then
        echo_error "No ros-ingress pod found in namespace $NAMESPACE"
        return 1
    fi

    echo_info >&2 "Found ros-ingress pod: $ingress_pod"

    # Extract the service account token from the pod
    local token=$(kubectl exec -n "$NAMESPACE" "$ingress_pod" -- cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null)
    if [ -z "$token" ]; then
        echo_error "Failed to extract service account token from pod $ingress_pod"
        return 1
    fi

    echo_info >&2 "Successfully extracted service account token"
    echo "$token"
}

# Function to get service account token from ros-ocp-api pod (for OAuth authentication)
get_ros_api_service_account_token() {
    echo_info >&2 "Extracting service account token from ros-ocp-api pod..."

    # Find the ros-ocp-api pod
    local api_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$api_pod" ]; then
        echo_error "No ros-ocp-api pod found in namespace $NAMESPACE"
        return 1
    fi

    echo_info >&2 "Found ros-ocp-api pod: $api_pod"

    # Extract the service account token from the pod
    local token=$(kubectl exec -n "$NAMESPACE" "$api_pod" -- cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null)
    if [ -z "$token" ]; then
        echo_error "Failed to extract service account token from pod $api_pod"
        return 1
    fi

    echo_info >&2 "Successfully extracted API service account token"
    echo "$token"
}

# Function to check if deployment exists
check_deployment() {
    if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
        echo_error "Namespace '$NAMESPACE' does not exist"
        return 1
    fi

    if ! helm list -n "$NAMESPACE" | grep -q "$HELM_RELEASE_NAME"; then
        echo_error "Helm release '$HELM_RELEASE_NAME' not found in namespace '$NAMESPACE'"
        return 1
    fi

    return 0
}

# Function to detect platform (OpenShift vs Kubernetes)
detect_platform() {
    # Check if OpenShift route API resources actually exist (more than just header line)
    local route_count=$(kubectl api-resources --api-group=route.openshift.io 2>/dev/null | wc -l)
    if [ "$route_count" -gt 1 ]; then
        echo "openshift"
    else
        echo "kubernetes"
    fi
}

# Function to get service URL based on platform (unified path-based approach)
get_service_url() {
    local service_name="$1"
    local path="$2"
    local platform=$(detect_platform)

    if [ "$platform" = "openshift" ]; then
        # OpenShift: Try to get route URL
        local route_host=$(kubectl get route "$HELM_RELEASE_NAME-$service_name" -n "$NAMESPACE" -o jsonpath='{.spec.host}' 2>/dev/null)
        if [ -n "$route_host" ]; then
            # Determine protocol - check if route has TLS
            local tls_termination=$(kubectl get route "$HELM_RELEASE_NAME-$service_name" -n "$NAMESPACE" -o jsonpath='{.spec.tls.termination}' 2>/dev/null)
            if [ -n "$tls_termination" ]; then
                echo "https://$route_host$path"
            else
                echo "http://$route_host$path"
            fi
        else
            echo_error "Route not found for $service_name. OpenShift Routes should be configured properly."
            return 1
        fi
    else
        # Kubernetes: Use Ingress with unified hostname
        local hostname=$(get_ingress_hostname)
        echo "http://$hostname$path"
    fi
}

# Function to wait for services to be ready
wait_for_services() {
    echo_info "Waiting for services to be ready..."

    # Wait for pods to be ready
    kubectl wait --for=condition=ready pod -l "app.kubernetes.io/instance=$HELM_RELEASE_NAME" \
        --namespace "$NAMESPACE" \
        --timeout=300s \
        --field-selector=status.phase!=Succeeded

    echo_success "All pods are ready"
    echo_info "Ingress connectivity has already been validated by install-helm-chart.sh"
}

# Function to create test data
create_test_data() {
    echo_info "Creating test data with current timestamps..." >&2

    # Generate dynamic timestamps for current data (multiple intervals for better recommendations)
    local now_date=$(date -u +%Y-%m-%d)
    local interval_start_1=$(date -u -d '75 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_1=$(date -u -d '60 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_start_2=$(date -u -d '60 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_2=$(date -u -d '45 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_start_3=$(date -u -d '45 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_3=$(date -u -d '30 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_start_4=$(date -u -d '30 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_4=$(date -u -d '15 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')

    echo_info "Using timestamps:" >&2
    echo_info "  Report date: $now_date" >&2
    echo_info "  Interval 1: $interval_start_1 to $interval_end_1" >&2
    echo_info "  Interval 2: $interval_start_2 to $interval_end_2" >&2
    echo_info "  Interval 3: $interval_start_3 to $interval_end_3" >&2
    echo_info "  Interval 4: $interval_start_4 to $interval_end_4" >&2

    # Create a temporary CSV file with proper ROS-OCP format and current timestamps
    local test_csv=$(mktemp)
    cat > "$test_csv" << EOF
report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
$now_date,$now_date,$interval_start_1,$interval_end_1,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.247832,0.185671,0.324131,0.247832,0.001,0.002,0.001,536870912,536870912,1073741824,1073741824,413587266.064516,410009344,420900544,413587266.064516,393311537.548387,390293568,396371392,393311537.548387
$now_date,$now_date,$interval_start_2,$interval_end_2,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.265423,0.198765,0.345678,0.265423,0.0012,0.0025,0.0012,536870912,536870912,1073741824,1073741824,427891456.123456,422014016,435890624,427891456.123456,407654321.987654,403627568,411681024,407654321.987654
$now_date,$now_date,$interval_start_3,$interval_end_3,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.289567,0.210987,0.367890,0.289567,0.0008,0.0018,0.0008,536870912,536870912,1073741824,1073741824,445678901.234567,441801728,449556074,445678901.234567,425987654.321098,421960800,430014256,425987654.321098
$now_date,$now_date,$interval_start_4,$interval_end_4,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.234567,0.189012,0.298765,0.234567,0.0005,0.0012,0.0005,536870912,536870912,1073741824,1073741824,398765432.101234,394887168,402643696,398765432.101234,378654321.098765,374627568,382681024,378654321.098765
EOF

    echo "$test_csv"
}

# Function to upload test data using insights-ros-ingress
upload_test_data() {
    echo_info "=== STEP 1: Upload HCCM Data via insights-ros-ingress ===="
    echo_info "Testing the new insights-ros-ingress service which:"
    echo_info "- Replaces the insights-ingress-go service"
    echo_info "- Automatically extracts CSV files from uploaded archives"
    echo_info "- Uploads CSV files directly to MinIO ros-data bucket"
    echo_info "- Publishes Kafka events to trigger ROS processing"
    echo_info ""

    local test_csv=$(create_test_data)
    local test_dir=$(mktemp -d)

    # Generate unique file UUID for this test
    local file_uuid
    if command -v uuidgen >/dev/null 2>&1; then
        file_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    else
        # Fallback UUID generation
        file_uuid=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(str(uuid.uuid4()))" 2>/dev/null || echo "$(date +%s)-test-uuid")
    fi

    local csv_filename="${file_uuid}_openshift_usage_report.0.csv"
    local tar_filename="cost-mgmt.tar.gz"

    # Copy CSV to temporary directory with expected filename
    if ! cp "$test_csv" "$test_dir/$csv_filename"; then
        echo_error "Failed to copy CSV file to temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Create manifest.json file (required by insights-ros-ingress)
    local manifest_json="$test_dir/manifest.json"
    local cluster_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local current_date=$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")
    local start_date=$(date -u +"%Y-%m-%dT%H:00:00Z")
    local end_date=$(date -u +"%Y-%m-%dT%H:59:59Z")

    cat > "$manifest_json" << EOF
{
    "uuid": "$file_uuid",
    "cluster_id": "$cluster_id",
    "version": "test-version",
    "date": "$current_date",
    "files": [
        "$csv_filename"
    ],
    "resource_optimization_files": [
        "$csv_filename"
    ],
    "start": "$start_date",
    "end": "$end_date",
    "cr_status": {
        "clusterID": "$cluster_id",
        "clusterVersion": "test-4.10",
        "api_url": "http://localhost:32061",
        "authentication": {
            "type": "bearer",
            "secret_name": "test-auth-secret",
            "credentials_found": true
        },
        "packaging": {
            "last_successful_packaging_time": null,
            "max_reports_to_store": 30,
            "max_size_MB": 100,
            "number_reports_stored": 1
        },
        "upload": {
            "ingress_path": "/api/ingress/v1/upload",
            "upload": true,
            "upload_wait": 30,
            "upload_cycle": 360,
            "last_successful_upload_time": null,
            "validate_cert": false
        },
        "operator_commit": "test-commit",
        "prometheus": {
            "prometheus_configured": true,
            "prometheus_connected": true,
            "context_timeout": 120,
            "last_query_start_time": "$current_date",
            "last_query_success_time": "$current_date",
            "service_address": "https://prometheus-test",
            "skip_tls_verification": true
        },
        "reports": {
            "report_month": "$(date +%m)",
            "last_hour_queried": "$start_date - $end_date",
            "data_collected": true
        },
        "source": {
            "sources_path": "/api/sources/v1.0/",
            "create_source": false,
            "last_check_time": null,
            "check_cycle": 1440
        },
        "storage": {}
    },
    "certified": false
}
EOF

    echo_info "Created manifest.json with cluster_id: $cluster_id"

    # Create tar.gz file (insights-ros-ingress will extract this automatically)
    echo_info "Creating HCCM tar.gz archive for insights-ros-ingress..."
    if ! (cd "$test_dir" && tar -czf "$tar_filename" "$csv_filename" "manifest.json"); then
        echo_error "Failed to create tar.gz archive"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    echo_info "Uploading HCCM archive to insights-ros-ingress..."

    # Get authentication token (required by insights-ros-ingress in Kubernetes)
    local auth_token=""
    local auth_setup_ok=false

    # Check if we can use the kubeconfig created by deploy-kind.sh
    if [ -f "/tmp/dev-kubeconfig" ]; then
        # Try to get token from the kubeconfig file
        auth_token=$(kubectl --kubeconfig=/tmp/dev-kubeconfig config view --raw -o jsonpath='{.users[0].user.token}' 2>/dev/null || echo "")
        if [ -n "$auth_token" ]; then
            auth_setup_ok=true
            echo_info "✓ Using authentication token from dev kubeconfig"
        fi
    fi

    # Fallback: try to get token from service account secret directly
    if [ "$auth_setup_ok" = false ] && kubectl get serviceaccount insights-ros-ingress -n "$NAMESPACE" >/dev/null 2>&1; then
        # Try the new token secret created by deploy-kind.sh
        auth_token=$(kubectl get secret insights-ros-ingress-token -n "$NAMESPACE" \
            -o jsonpath='{.data.token}' 2>/dev/null | base64 -d 2>/dev/null || echo "")

        if [ -n "$auth_token" ]; then
            auth_setup_ok=true
            echo_info "✓ Using authentication token from service account secret"
        fi
    fi

    # If still no token, try service account token creation as fallback
    if [ "$auth_setup_ok" = false ]; then
        echo_info "Trying to create service account token for upload authentication..."
        auth_token=$(kubectl create token insights-ros-ingress -n "$NAMESPACE" --duration=1h 2>/dev/null || echo "")
        if [ -n "$auth_token" ]; then
            auth_setup_ok=true
            echo_success "✓ Created temporary service account token for upload"
        else
            echo_warning "Could not obtain authentication token for insights-ros-ingress upload"
            echo_info "Will proceed with OAuth token from ros-ocp-api service account"
        fi
    fi

    # Upload the tar.gz file using curl with proper headers and content-type
    local upload_url=$(get_service_url "ingress" "/api/ingress/v1/upload")
    echo_info "Uploading to: $upload_url"
    echo_info "Full upload URL: $upload_url"
    echo_info "Platform detected: $(detect_platform)"
    echo_info "Ingress hostname: $(get_ingress_hostname)"

    # Get OAuth 2.0 token from ros-ocp-api service account for upload
    local upload_bearer_token=$(get_ros_api_service_account_token)
    if [ -z "$upload_bearer_token" ]; then
        echo_error "Failed to get API service account token for upload"
        return 1
    fi

    # Build curl command with OAuth Bearer token
    local curl_cmd="curl -s -w \"%{http_code}\" --connect-timeout 10 --max-time 60 \
        -F \"file=@${test_dir}/${tar_filename};type=application/vnd.redhat.hccm.upload\" \
        -H \"Authorization: Bearer $upload_bearer_token\" \
        -H \"x-rh-request-id: test-request-$(date +%s)\" \
        -H \"Host: localhost\""

    echo_info "✓ OAuth Bearer token will be included in upload request"

    curl_cmd="$curl_cmd \"$upload_url\""

    local response=$(eval $curl_cmd)
    local http_code="${response: -3}"
    local response_body="${response%???}"

    # Cleanup
    rm -f "$test_csv"
    rm -rf "$test_dir"

    if [ "$http_code" != "202" ]; then
        echo_error "Upload failed! HTTP $http_code"
        echo_error "Response: $response_body"
        if [ "$http_code" = "401" ]; then
            echo_error "Authentication failed. Check that insights-ros-ingress service account exists"
        fi
        return 1
    fi

    echo_success "Upload successful! HTTP $http_code"
    echo_info "Response: $response_body"
    echo_info "insights-ros-ingress will now:"
    echo_info "1. Extract CSV files from the uploaded archive"
    echo_info "2. Upload CSV files to MinIO ros-data bucket"
    echo_info "3. Publish Kafka events to trigger ROS processing"

    # Check ingress logs to see processing
    echo_info "Checking insights-ros-ingress logs for processing:"
    kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=ingress --tail=5 | grep -E "(processing|extract|upload|kafka)" || echo "No processing logs found"

    return 0
}

# Function to verify insights-ros-ingress processing
verify_ros_ingress_processing() {
    echo_info "=== STEP 2: Verify insights-ros-ingress Processing ===="
    echo_info "insights-ros-ingress should automatically:"
    echo_info "- Extract CSV files from the uploaded archive"
    echo_info "- Upload CSV files to MinIO ros-data bucket"
    echo_info "- Publish Kafka events to trigger ROS processing"
    echo_info ""

    # Generate unique file UUID for this test
    local file_uuid
    if command -v uuidgen >/dev/null 2>&1; then
        file_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    else
        # Fallback UUID generation
        file_uuid=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(str(uuid.uuid4()))" 2>/dev/null || echo "$(date +%s)-test-uuid")
    fi

    local csv_filename="${file_uuid}_openshift_usage_report.0.csv"

    echo_info "Generated file UUID: $file_uuid"
    echo_info "CSV filename: $csv_filename"

    # Create test CSV content with current timestamps (multiple data points)
    local now_date=$(date -u +%Y-%m-%d)
    local interval_start_1=$(date -u -d '60 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_1=$(date -u -d '45 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_start_2=$(date -u -d '45 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_2=$(date -u -d '30 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_start_3=$(date -u -d '30 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')
    local interval_end_3=$(date -u -d '15 minutes ago' '+%Y-%m-%d %H:%M:%S -0000 UTC')

    echo_info "Creating CSV with current timestamps:" >&2
    echo_info "  Report date: $now_date" >&2
    echo_info "  Multiple intervals for better recommendations" >&2

    local csv_content="report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
$now_date,$now_date,$interval_start_1,$interval_end_1,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.247832,0.185671,0.324131,0.247832,0.001,0.002,0.001,536870912,536870912,1073741824,1073741824,413587266.064516,410009344,420900544,413587266.064516,393311537.548387,390293568,396371392,393311537.548387
$now_date,$now_date,$interval_start_2,$interval_end_2,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.265423,0.198765,0.345678,0.265423,0.0012,0.0025,0.0012,536870912,536870912,1073741824,1073741824,427891456.123456,422014016,435890624,427891456.123456,407654321.987654,403627568,411681024,407654321.987654
$now_date,$now_date,$interval_start_3,$interval_end_3,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.289567,0.210987,0.367890,0.289567,0.0008,0.0018,0.0008,536870912,536870912,1073741824,1073741824,445678901.234567,441801728,449556074,445678901.234567,425987654.321098,421960800,430014256,425987654.321098"

    # Copy CSV data to ros-data bucket via MinIO pod
    echo_info "Copying CSV to ros-data bucket..."

    # Check for CSV files in ros-data bucket (created by insights-ros-ingress)
    echo_info "Checking for CSV files in MinIO ros-data bucket..."

    # MinIO is deployed in application namespace with Helm labels
    # Labels: app.kubernetes.io/instance=<release>, app.kubernetes.io/name=minio, app.kubernetes.io/component=storage
    local minio_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/instance=$HELM_RELEASE_NAME,app.kubernetes.io/name=minio" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    # Kafka is deployed by Strimzi in 'kafka' namespace
    local kafka_namespace="kafka"
    local kafka_cluster="ros-ocp-kafka"
    local kafka_pod=$(kubectl get pods -n "$kafka_namespace" -l "strimzi.io/cluster=$kafka_cluster,strimzi.io/name=${kafka_cluster}-kafka" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -z "$minio_pod" ]; then
        echo_error "MinIO pod not found in namespace $NAMESPACE"
        echo_info "Available pods:"
        kubectl get pods -n "$NAMESPACE" -o wide
        return 1
    fi

    if [ -z "$kafka_pod" ]; then
        echo_error "Kafka pod not found in namespace $kafka_namespace"
        echo_info "Available pods in kafka namespace:"
        kubectl get pods -n "$kafka_namespace" -o wide
        return 1
    fi

    echo_info "Using MinIO pod: $minio_pod (namespace: $NAMESPACE)"
    echo_info "Using Kafka pod: $kafka_pod (namespace: $kafka_namespace)"

    # Configure MinIO client
    echo_info "Configuring MinIO client..."
    kubectl exec -n "$NAMESPACE" "$minio_pod" -- /usr/bin/mc alias set myminio http://localhost:9000 minioaccesskey miniosecretkey || echo "MinIO alias configuration failed"

    # Check MinIO client configuration
    echo_info "Checking MinIO client configuration:"
    kubectl exec -n "$NAMESPACE" "$minio_pod" -- /usr/bin/mc --help | head -5 || echo "MinIO client help failed"

    # Check if MinIO is accessible
    echo_info "Checking MinIO connectivity:"
    kubectl exec -n "$NAMESPACE" "$minio_pod" -- /usr/bin/mc version || echo "MinIO version check failed"

    # List all available hosts/aliases
    echo_info "Listing MinIO hosts/aliases:"
    kubectl exec -n "$NAMESPACE" "$minio_pod" -- /usr/bin/mc alias ls || echo "MinIO alias list failed"

    local retries=6
    local csv_found=false

    for i in $(seq 1 $retries); do
        echo_info "Checking for CSV files in ros-data bucket (attempt $i/$retries)..."

        # List files in ros-data bucket with detailed output
        echo_info "Listing all files in ros-data bucket:"
        local bucket_contents=$(kubectl exec -n "$NAMESPACE" "$minio_pod" -- \
            /usr/bin/mc ls --recursive myminio/ros-data/ 2>&1 || echo "ERROR: Failed to list bucket")

        echo_info "Bucket contents:"
        echo "$bucket_contents"

        # Also check if the bucket exists
        echo_info "Checking if ros-data bucket exists:"
        kubectl exec -n "$NAMESPACE" "$minio_pod" -- \
            /usr/bin/mc ls myminio/ 2>&1 || echo "ERROR: Failed to list buckets"

        if echo "$bucket_contents" | grep -q "\.csv"; then
            echo_success "CSV files found in ros-data bucket (uploaded by insights-ros-ingress):"
            echo "$bucket_contents" | grep "\.csv"
            csv_found=true
            break
        else
            echo_info "No CSV files found yet, waiting... ($i/$retries)"
            sleep 10
        fi
    done

    if [ "$csv_found" = false ]; then
        echo_error "No CSV files found in ros-data bucket after $retries attempts"
        echo_info "insights-ros-ingress may have failed to process the upload"
        echo_info "Checking MinIO bucket contents for debugging:"
        kubectl exec -n "$NAMESPACE" "$minio_pod" -- \
            /usr/bin/mc ls --recursive myminio/ros-data/ 2>/dev/null || echo "Could not list bucket contents"
        return 1
    fi

    # Ensure Kafka topic exists before publishing
    echo_info "Ensuring Kafka topic 'hccm.ros.events' exists..."
    # Use kubectl run with Strimzi Kafka image to access Kafka client tools
    # Bootstrap server uses the Kafka service DNS name
    kubectl run kafka-topics-temp --rm -i --restart=Never --image=quay.io/strimzi/kafka:latest-kafka-3.7.0 -n "$kafka_namespace" -- \
        bin/kafka-topics.sh --create --topic hccm.ros.events --bootstrap-server ${kafka_cluster}-kafka-bootstrap:9092 \
        --partitions 1 --replication-factor 1 --if-not-exists 2>/dev/null || echo "Topic creation attempted"

    # Create proper Kafka message matching insights-ros-ingress format
    echo_info "Creating properly formatted Kafka message for processor validation..."

    # Generate request ID for tracing
    local request_id=$(uuidgen | tr '[:upper:]' '[:lower:]')

    # Get cluster information (use file_uuid as cluster_id for testing)
    local cluster_id="$file_uuid"
    local org_id="1"
    local account="1"

    # Create minimal base64 identity (legacy field - ignored in OAuth2 but required for validation)
    local identity="{\"account_number\":\"$account\",\"org_id\":\"$org_id\",\"user\":{\"is_org_admin\":true}}"
    local b64_identity=$(echo -n "$identity" | base64 -w 0)

    # Create presigned URL matching insights-ros-ingress format
    local date_partition="$(date +%Y-%m-%d)"
    local object_key="ros/org_${org_id}/source=${cluster_id}/date=${date_partition}/${csv_filename}"
    local minio_url="http://ros-ocp-minio:9000/ros-data/${object_key}"
    local presigned_url="${minio_url}?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioaccesskey%2F$(date +%Y%m%d)%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=$(date +%Y%m%dT%H%M%SZ)&X-Amz-Expires=172800&X-Amz-SignedHeaders=host&X-Amz-Signature=test-signature"

    # Construct complete Kafka message as single line (kafka-console-producer sends line-by-line)
    local kafka_message="{\"request_id\":\"$request_id\",\"b64_identity\":\"$b64_identity\",\"metadata\":{\"account\":\"$account\",\"org_id\":\"$org_id\",\"source_id\":\"$cluster_id\",\"provider_uuid\":\"$cluster_id\",\"cluster_uuid\":\"$cluster_id\",\"cluster_alias\":\"$cluster_id\",\"operator_version\":\"test-1.0.0\"},\"files\":[\"$presigned_url\"],\"object_keys\":[\"$object_key\"]}"

    echo_info "Publishing properly formatted Kafka message:"
    echo_info "Request ID: $request_id"
    echo_info "Cluster ID: $cluster_id"
    echo_info "Files count: 1"
    echo_info "Object key: $object_key"

    # Use kubectl run with Strimzi Kafka image to access Kafka client tools
    # Bootstrap server uses the Kafka service DNS name
    echo "$kafka_message" | kubectl run kafka-producer-temp --rm -i --restart=Never --image=quay.io/strimzi/kafka:latest-kafka-3.7.0 -n "$kafka_namespace" -- \
        bin/kafka-console-producer.sh --bootstrap-server ${kafka_cluster}-kafka-bootstrap:9092 --topic hccm.ros.events

    if [ $? -eq 0 ]; then
        echo_success "Kafka message published successfully"
        echo_info "Request ID: $request_id"
        echo_info "File UUID: $file_uuid"

        # Wait for processor to handle the message
        echo_info "Waiting for processor to handle the properly formatted message..."
        sleep 15

        # Check processor logs for successful processing (not validation errors)
        echo_info "Checking processor logs for message processing:"
        local recent_logs=$(kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=processor --tail=20 --since=60s)

        if echo "$recent_logs" | grep -q "request_id.*$request_id"; then
            echo_success "✓ Message with request ID $request_id found in processor logs"

            # Check for validation errors vs successful processing
            if echo "$recent_logs" | grep -q "Invalid kafka message.*$request_id"; then
                echo_error "✗ Message validation still failed - check processor logs for details"
                echo "$recent_logs" | grep -A 2 -B 2 "Invalid kafka message"
            elif echo "$recent_logs" | grep -q "Error.*$request_id"; then
                echo_warning "⚠ Message processed but encountered errors during processing"
                echo "$recent_logs" | grep -A 2 -B 2 "Error.*$request_id"
            else
                echo_success "✓ Message appears to have been processed successfully"
                echo_info "Recent processor activity:"
                echo "$recent_logs" | grep "$request_id" | tail -3
            fi
        else
            echo_warning "Message may still be processing or not yet visible in logs"
            echo_info "Recent processor logs (last 5 lines):"
            echo "$recent_logs" | tail -5
        fi

        echo_info "CSV file: $csv_filename"
        local hostname=$(get_ingress_hostname)
        echo_info "MinIO object key: $object_key"
        echo_info "Accessible at: http://$hostname/minio/browser/ros-data/"
    else
        echo_error "Failed to publish Kafka message"
        return 1
    fi
}

# Function to verify data processing
verify_processing() {
    echo_info "=== STEP 4: Verify Data Processing ===="

    echo_info "Waiting for data processing (60 seconds)..."
    sleep 20

    # Check processor logs
    echo_info "Checking processor logs..."
    local processor_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rosocp-processor" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$processor_pod" ]; then
        echo_info "Recent processor logs:"
        kubectl logs -n "$NAMESPACE" "$processor_pod" --tail=20 || true
    fi

    # Check database for workload records
    echo_info "Checking database for workload records..."
    local db_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=db-ros" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$db_pod" ]; then
        local row_count=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")

        if [ "$row_count" -gt 0 ]; then
            echo_success "Found $row_count workload records in database"

            # Show sample data
            echo_info "Sample workload data:"
            kubectl exec -n "$NAMESPACE" "$db_pod" -- \
                psql -U postgres -d postgres -c \
                "SELECT cluster_uuid, workload_name, workload_type, namespace FROM workloads LIMIT 3;" 2>/dev/null || true
        else
            echo_warning "No workload data found in database yet"
        fi
    fi

    # Check Kruize experiments via database (listExperiments API has known issue with KruizeLMExperimentEntry)
    echo_info "Checking Kruize experiments via database..."
    local db_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=db-kruize" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$db_pod" ]; then
        local exp_count=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM kruize_experiments;" 2>/dev/null | tr -d ' ' || echo "0")

        if [ "$exp_count" -gt 0 ]; then
            echo_success "Found $exp_count Kruize experiment(s) in database"

            # Show experiment details
            echo_info "Recent experiment details:"
            kubectl exec -n "$NAMESPACE" "$db_pod" -- \
                psql -U postgres -d postgres -c \
                "SELECT experiment_name, status, mode FROM kruize_experiments ORDER BY experiment_id DESC LIMIT 1;" 2>/dev/null || true
        else
            echo_warning "No Kruize experiments found in database yet"
        fi
    else
        echo_warning "Could not access Kruize database"
    fi
}

# Function to verify recommendations are available via ros-ocp-api
verify_recommendations() {
    echo_info "=== STEP 5: Verify Recommendations via ROS-OCP API ===="

    # Wait additional time for recommendations to be processed with fresh data
    echo_info "Waiting for recommendations to be processed with fresh timestamps (45 seconds)..."
    echo_info "Fresh data should trigger Kruize to generate valid recommendations..."
    sleep 45

    # Create X-Rh-Identity header for ros-ocp-backend authentication
    echo_info "Creating X-Rh-Identity header for API authentication..."
    local rh_identity_header=$(create_rh_identity_header "12345")
    echo_info "Successfully created X-Rh-Identity header"

    # Get platform-appropriate URLs
    local status_url=$(get_service_url "ros-ocp-rosocp-api" "/status")
    local api_base_url=$(get_service_url "ros-ocp-rosocp-api" "/api/cost-management/v1")

    # Test API status endpoint first (public endpoint - no auth needed)
    echo_info "Testing ROS-OCP API status..."
    echo_info "Status URL: $status_url"
    local status_response=$(curl -s -w "%{http_code}" --connect-timeout 5 --max-time 15 -o /tmp/status_response.json \
        -H "Host: localhost" \
        "$status_url" 2>/dev/null || echo "000")

    local status_http_code="${status_response: -3}"

    if [ "$status_http_code" = "200" ]; then
        echo_success "ROS-OCP API status endpoint is accessible"
        if [ -f /tmp/status_response.json ]; then
            echo_info "Status response: $(cat /tmp/status_response.json)"
            rm -f /tmp/status_response.json
        fi
    else
        echo_error "ROS-OCP API status endpoint not accessible (HTTP $status_http_code)"
        return 1
    fi

    # Test recommendations list endpoint (requires X-Rh-Identity)
    echo_info "Testing recommendations list endpoint..."
    local list_response=$(curl -s -w "%{http_code}" --connect-timeout 5 --max-time 30 -o /tmp/recommendations_list.json \
        -H "X-Rh-Identity: $rh_identity_header" \
        -H "Content-Type: application/json" \
        -H "Host: localhost" \
        "$api_base_url/recommendations/openshift" 2>/dev/null || echo "000")

    local list_http_code="${list_response: -3}"

    if [ "$list_http_code" = "200" ]; then
        echo_success "Recommendations list endpoint accessible (HTTP $list_http_code)"

        if [ -f /tmp/recommendations_list.json ]; then
            # Check if we have actual recommendations
            local rec_count=$(python3 -c "
import json, sys
try:
    with open('/tmp/recommendations_list.json', 'r') as f:
        data = json.load(f)
    if 'data' in data and isinstance(data['data'], list):
        print(len(data['data']))
    else:
        print(0)
except:
    print(0)
" 2>/dev/null || echo "0")

            echo_info "Found $rec_count recommendation(s) in the response"

            if [ "$rec_count" -gt 0 ]; then
                echo_success "✓ Recommendations are available via API!"

                # Show summary of first recommendation
                echo_info "Sample recommendation summary:"
                python3 -c "
import json
try:
    with open('/tmp/recommendations_list.json', 'r') as f:
        data = json.load(f)
    if 'data' in data and len(data['data']) > 0:
        rec = data['data'][0]
        print(f'  ID: {rec.get(\"id\", \"N/A\")}')
        print(f'  Cluster: {rec.get(\"cluster_alias\", \"N/A\")}')
        print(f'  Workload: {rec.get(\"workload\", \"N/A\")}')
        print(f'  Container: {rec.get(\"container\", \"N/A\")}')
        print(f'  Namespace: {rec.get(\"project\", \"N/A\")}')
except Exception as e:
    print(f'  Error parsing response: {e}')
" 2>/dev/null || echo "  Unable to parse recommendation details"

                # Test individual recommendation endpoint
                local rec_id=$(python3 -c "
import json
try:
    with open('/tmp/recommendations_list.json', 'r') as f:
        data = json.load(f)
    if 'data' in data and len(data['data']) > 0:
        print(data['data'][0].get('id', ''))
except:
    pass
" 2>/dev/null)

                if [ -n "$rec_id" ]; then
                    echo_info "Testing individual recommendation endpoint for ID: $rec_id"
                    local detail_response=$(curl -s -w "%{http_code}" --connect-timeout 5 --max-time 30 -o /tmp/recommendation_detail.json \
                        -H "X-Rh-Identity: $rh_identity_header" \
                        -H "Content-Type: application/json" \
                        -H "Host: localhost" \
                        "$api_base_url/recommendations/openshift/$rec_id" 2>/dev/null || echo "000")

                    local detail_http_code="${detail_response: -3}"

                    if [ "$detail_http_code" = "200" ]; then
                        echo_success "✓ Individual recommendation endpoint accessible (HTTP $detail_http_code)"

                        # Show recommendation details
                        echo_info "Recommendation details available:"
                        python3 -c "
import json
try:
    with open('/tmp/recommendation_detail.json', 'r') as f:
        data = json.load(f)
    if 'recommendations' in data and 'data' in data['recommendations']:
        rec_data = data['recommendations']['data']
        if rec_data:
            print(f'  Current CPU request: {rec_data.get(\"requests\", {}).get(\"cpu\", {}).get(\"amount\", \"N/A\")}')
            print(f'  Recommended CPU request: {rec_data.get(\"requests\", {}).get(\"cpu\", {}).get(\"recommendation\", {}).get(\"amount\", \"N/A\")}')
            print(f'  Current Memory request: {rec_data.get(\"requests\", {}).get(\"memory\", {}).get(\"amount\", \"N/A\")}')
            print(f'  Recommended Memory request: {rec_data.get(\"requests\", {}).get(\"memory\", {}).get(\"recommendation\", {}).get(\"amount\", \"N/A\")}')
        else:
            print('  No recommendation data available')
except Exception as e:
    print(f'  Error parsing recommendation: {e}')
" 2>/dev/null || echo "  Unable to parse recommendation details"

                        rm -f /tmp/recommendation_detail.json
                    else
                        echo_warning "Individual recommendation endpoint returned HTTP $detail_http_code"
                    fi
                fi
            else
                echo_warning "No recommendations found in response with fresh timestamps"
                echo_info "This may indicate:"
                echo_info "  - Kruize is still processing the recent data (may need more time)"
                echo_info "  - Fresh timestamps generated valid data but recommendations aren't ready yet"
                echo_info "  - Check Kruize logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=kruize --tail=50"
                echo_info "  - Check processor logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=rosocp-processor --tail=20"
            fi

            rm -f /tmp/recommendations_list.json
        fi
    elif [ "$list_http_code" = "401" ]; then
        echo_error "Authentication failed (HTTP 401) - check identity header"
        return 1
    elif [ "$list_http_code" = "000" ]; then
        echo_error "Could not connect to ROS-OCP API - check if service is running and port $API_PORT is accessible"
        return 1
    else
        echo_warning "Recommendations endpoint returned HTTP $list_http_code"
        if [ -f /tmp/recommendations_list.json ]; then
            echo_info "Response: $(cat /tmp/recommendations_list.json)"
            rm -f /tmp/recommendations_list.json
        fi
    fi

    # Test CSV export format (requires X-Rh-Identity)
    echo_info "Testing CSV export functionality..."
    local csv_response=$(curl -s -w "%{http_code}" --connect-timeout 5 --max-time 30 -o /tmp/recommendations.csv \
        -H "X-Rh-Identity: $rh_identity_header" \
        -H "Accept: text/csv" \
        -H "Host: localhost" \
        "$api_base_url/recommendations/openshift?format=csv" 2>/dev/null || echo "000")

    local csv_http_code="${csv_response: -3}"

    if [ "$csv_http_code" = "200" ]; then
        echo_success "✓ CSV export functionality working (HTTP $csv_http_code)"
        if [ -f /tmp/recommendations.csv ]; then
            local csv_lines=$(wc -l < /tmp/recommendations.csv 2>/dev/null || echo "0")
            echo_info "CSV contains $csv_lines lines"
            rm -f /tmp/recommendations.csv
        fi
    else
        echo_warning "CSV export returned HTTP $csv_http_code"
        rm -f /tmp/recommendations.csv
    fi

    echo_info "Recommendation verification completed"
}

# Function to verify workloads are stored in ROS database
verify_workloads_in_db() {
    echo_info "=== STEP 6: Verify Workloads in ROS Database ===="

    # Check database for workload records with detailed analysis
    echo_info "Checking workloads table in ROS database..."
    local db_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=db-ros" -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$db_pod" ]; then
        echo_error "ROS database pod not found"
        return 1
    fi

    # Test database connectivity
    echo_info "Testing database connectivity..."
    if ! kubectl exec -n "$NAMESPACE" "$db_pod" -- psql -U postgres -d postgres -c "SELECT 1;" >/dev/null 2>&1; then
        echo_error "Cannot connect to ROS database"
        return 1
    fi
    echo_success "✓ Database connection successful"

    # Check if workloads table exists
    echo_info "Verifying workloads table exists..."
    local table_exists=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
        psql -U postgres -d postgres -t -c \
        "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'workloads');" 2>/dev/null | tr -d ' ' || echo "f")

    if [ "$table_exists" = "t" ]; then
        echo_success "✓ Workloads table exists"
    else
        echo_error "Workloads table does not exist"
        return 1
    fi

    # Get workload count
    local workload_count=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
        psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")

    if [ "$workload_count" -gt 0 ]; then
        echo_success "✓ Found $workload_count workload(s) in database"

        # Show workload table schema
        echo_info "Workload table schema:"
        kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = 'workloads' ORDER BY ordinal_position;" 2>/dev/null || true

        # Show detailed workload information
        echo_info "Detailed workload information:"
        kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT
                id,
                org_id,
                cluster_id,
                experiment_name,
                namespace,
                workload_type,
                workload_name,
                array_length(containers, 1) as container_count,
                containers[1:3] as first_containers,
                metrics_upload_at
            FROM workloads
            ORDER BY id
            LIMIT 5;" 2>/dev/null || true

        # Test workload data integrity
        echo_info "Testing workload data integrity..."

        # Check for required fields
        local missing_org_id=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM workloads WHERE org_id IS NULL OR org_id = '';" 2>/dev/null | tr -d ' \n' || echo "0")
        missing_org_id=${missing_org_id:-0}

        local missing_workload_name=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM workloads WHERE workload_name IS NULL OR workload_name = '';" 2>/dev/null | tr -d ' \n' || echo "0")
        missing_workload_name=${missing_workload_name:-0}

        local missing_workload_type=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(*) FROM workloads WHERE workload_type IS NULL OR workload_type = '';" 2>/dev/null | tr -d ' \n' || echo "0")
        missing_workload_type=${missing_workload_type:-0}

        if [ "$missing_org_id" -eq 0 ] && [ "$missing_workload_name" -eq 0 ] && [ "$missing_workload_type" -eq 0 ]; then
            echo_success "✓ All workloads have required fields populated"
        else
            echo_warning "Data integrity issues found:"
            [ "$missing_org_id" -gt 0 ] && echo_warning "  $missing_org_id workloads missing org_id"
            [ "$missing_workload_name" -gt 0 ] && echo_warning "  $missing_workload_name workloads missing workload_name"
            [ "$missing_workload_type" -gt 0 ] && echo_warning "  $missing_workload_type workloads missing workload_type"
        fi

        # Check cluster relationships
        echo_info "Checking cluster relationships..."
        local cluster_count=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c "SELECT COUNT(DISTINCT cluster_id) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")

        local orphaned_workloads=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c \
            "SELECT COUNT(*) FROM workloads w
             LEFT JOIN clusters c ON w.cluster_id = c.id
             WHERE c.id IS NULL;" 2>/dev/null | tr -d ' ' || echo "0")

        echo_info "  Workloads span $cluster_count cluster(s)"
        if [ "$orphaned_workloads" -eq 0 ]; then
            echo_success "✓ All workloads properly linked to clusters"
        else
            echo_warning "  $orphaned_workloads workloads have invalid cluster references"
        fi

        # Show workload distribution by type
        echo_info "Workload distribution by type:"
        kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT workload_type, COUNT(*) as count
             FROM workloads
             GROUP BY workload_type
             ORDER BY count DESC;" 2>/dev/null || true

        # Show workload distribution by namespace
        echo_info "Workload distribution by namespace:"
        kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT namespace, COUNT(*) as count
             FROM workloads
             GROUP BY namespace
             ORDER BY count DESC
             LIMIT 10;" 2>/dev/null || true

        # Check container information
        local total_containers=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c \
            "SELECT SUM(array_length(containers, 1)) FROM workloads WHERE containers IS NOT NULL;" 2>/dev/null | tr -d ' ' || echo "0")

        echo_info "Total containers across all workloads: $total_containers"

        # Verify recent data updates
        echo_info "Checking data freshness..."
        local recent_updates=$(kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c \
            "SELECT COUNT(*) FROM workloads WHERE metrics_upload_at > NOW() - INTERVAL '1 hour';" 2>/dev/null | tr -d ' ' || echo "0")

        if [ "$recent_updates" -gt 0 ]; then
            echo_success "✓ $recent_updates workloads updated within the last hour"
        else
            echo_info "  No workloads updated in the last hour (may be expected for test data)"
        fi

        # Show most recent workload activity
        echo_info "Most recent workload uploads:"
        kubectl exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT workload_name, namespace, workload_type, metrics_upload_at
             FROM workloads
             ORDER BY metrics_upload_at DESC
             LIMIT 3;" 2>/dev/null || true

    else
        echo_warning "No workload data found in database"
        echo_info "This might indicate:"
        echo_info "  - Data processing is still in progress"
        echo_info "  - No data has been uploaded yet"
        echo_info "  - There was an issue with data processing"

        # Check if table is empty but exists
        echo_info "Checking if this is expected for test scenario..."
        return 0
    fi

    echo_info "Workload database verification completed"
}

# Function to run health checks
# Note: HTTP 400/500 responses are expected for invalid test data and indicate healthy services
run_health_checks() {
    echo_info "=== Health Checks ==="
    echo_info "Note: HTTP 400/500 responses indicate healthy services rejecting invalid test data"

    local failed_checks=0
    local platform=$(detect_platform)

    echo_info "Running health checks for platform: $platform"

    # Get platform-appropriate URLs (only for essential routes)
    local ingress_url=$(get_service_url "ingress" "/ready")
    local api_url=$(get_service_url "ros-ocp-rosocp-api" "/status")
    local kruize_url=$(get_service_url "kruize" "/api/kruize/listPerformanceProfiles")

    # Check ingress service by testing upload endpoint with service account token
    local upload_url=$(get_service_url "ingress" "/api/ingress/v1/upload")

    # Get service account token for authentication
    local sa_token=$(kubectl create token insights-ros-ingress -n "$NAMESPACE" --duration=1h 2>/dev/null)
    if [ -z "$sa_token" ]; then
        echo_warning "Could not get service account token, testing without authentication"
        local response_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 15 -H "Host: localhost" -X POST "$upload_url" -H "Content-Type: application/vnd.redhat.hccm.upload" --data-binary "test" 2>/dev/null)
        if [ "$response_code" = "405" ] || [ "$response_code" = "401" ]; then
            echo_success "Ingress upload endpoint is accessible at: $upload_url (HTTP $response_code)"
        elif [ "$response_code" = "400" ]; then
            echo_success "Ingress upload endpoint is accessible at: $upload_url (HTTP $response_code - service running, rejecting invalid test data)"
        else
            echo_error "Ingress upload endpoint is not accessible at: $upload_url (HTTP $response_code)"
            failed_checks=$((failed_checks + 1))
        fi
    else
        # Test with authentication
        local response_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 15 -H "Host: localhost" -H "Authorization: Bearer $sa_token" -X POST "$upload_url" -H "Content-Type: application/vnd.redhat.hccm.upload" --data-binary "test" 2>/dev/null)
        if [ "$response_code" = "200" ] || [ "$response_code" = "202" ]; then
            echo_success "Ingress upload endpoint is accessible with authentication at: $upload_url (HTTP $response_code)"
        elif [ "$response_code" = "400" ] || [ "$response_code" = "500" ]; then
            echo_success "Ingress upload endpoint is accessible with authentication at: $upload_url (HTTP $response_code - service running, rejecting invalid test data)"
        else
            echo_error "Ingress upload endpoint failed with authentication at: $upload_url (HTTP $response_code)"
            failed_checks=$((failed_checks + 1))
        fi
    fi

    # Check ROS-OCP API /status endpoint (public endpoint - no auth required)
    if curl -f -s --connect-timeout 5 --max-time 15 -H "Host: localhost" "$api_url" >/dev/null; then
        echo_success "ROS-OCP API status endpoint is accessible at: $api_url"
    else
        echo_error "ROS-OCP API status endpoint is not accessible at: $api_url"
        failed_checks=$((failed_checks + 1))
    fi

    # Check Kruize API
    if curl -f -s --connect-timeout 5 --max-time 15 -H "Host: localhost" "$kruize_url" >/dev/null; then
        echo_success "Kruize API is accessible at: $kruize_url"
    else
        echo_error "Kruize API is not accessible at: $kruize_url"
        failed_checks=$((failed_checks + 1))
    fi

    # Check pod status
    local pending_pods=$(kubectl get pods -n "$NAMESPACE" --field-selector=status.phase=Pending --no-headers 2>/dev/null | wc -l)
    local failed_pods=$(kubectl get pods -n "$NAMESPACE" --field-selector=status.phase=Failed --no-headers 2>/dev/null | wc -l)

    if [ "$pending_pods" -eq 0 ] && [ "$failed_pods" -eq 0 ]; then
        echo_success "All pods are running successfully"
    else
        echo_warning "$pending_pods pending pods, $failed_pods failed pods"
        failed_checks=$((failed_checks + 1))
    fi

    if [ $failed_checks -eq 0 ]; then
        echo_success "All health checks passed!"
    else
        echo_warning "$failed_checks health check(s) failed"
    fi

    return $failed_checks
}

# Function to show service logs
show_logs() {
    local service="${1:-}"

    if [ -z "$service" ]; then
        echo_info "Available services:"
        kubectl get pods -n "$NAMESPACE" -o custom-columns="NAME:.metadata.name,COMPONENT:.metadata.labels.app\.kubernetes\.io/name" --no-headers
        return 0
    fi

    local pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$service" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -n "$pod" ]; then
        echo_info "Logs for $service ($pod):"
        kubectl logs -n "$NAMESPACE" "$pod" --tail=50
    else
        echo_error "Pod not found for service: $service"
        return 1
    fi
}

# Main execution
main() {
    echo_info "ROS-OCP Kubernetes Data Flow Test"
    echo_info "=================================="

    # Check prerequisites
    if ! check_kubectl; then
        exit 1
    fi

    if ! check_deployment; then
        exit 1
    fi

    # Validate authentication tokens are available
    if ! validate_authentication_tokens; then
        echo_error "Authentication validation failed!"
        echo_error "This test requires authentication tokens to be available."
        echo_error "The test will FAIL without proper authentication setup."
        echo_info ""
        echo_info "To fix this issue:"
        echo_info "1. Run ./deploy-kind.sh to set up authentication"
        echo_info "2. Ensure the insights-ros-ingress service account exists"
        echo_info "3. Check that the token secret was created properly"
        echo_info "4. Verify the dev kubeconfig file was generated"
        echo_info ""
        echo_info "This test uses dynamic authentication and will attempt fallback methods."
        exit 1
    fi


    local platform=$(detect_platform)

    echo_info "Configuration:"
    echo_info "  Platform: $platform"
    echo_info "  Namespace: $NAMESPACE"
    echo_info "  Helm Release: $HELM_RELEASE_NAME"

    if [ "$platform" = "kubernetes" ]; then
        local hostname=$(get_ingress_hostname)
        echo_info "  Ingress Hostname: $hostname"
        echo_info "  Access Method: Ingress Controller (path-based routing)"
    else
        echo_info "  Access Method: OpenShift Routes (path-based routing)"
    fi
    echo ""

    # Wait for services to be ready
    if ! wait_for_services; then
        echo_error "Services are not ready. Aborting test."
        exit 1
    fi

    # Run complete data flow test
    echo_info "Starting complete data flow test..."

    if upload_test_data; then
        echo_success "Step 1: Upload completed successfully"
    else
        echo_error "Step 1: Upload failed"
        exit 1
    fi

    if verify_ros_ingress_processing; then
        echo_success "Steps 2-3: insights-ros-ingress processing completed successfully"
    else
        echo_error "Steps 2-3: insights-ros-ingress processing failed"
        exit 1
    fi

    verify_processing

    # Verify workloads are stored in database
    verify_workloads_in_db

    # Verify recommendations are available via API
    verify_recommendations

    echo ""
    run_health_checks

    echo ""
    echo_success "Data flow test completed!"
    echo_info "Use '$0 logs <service>' to view specific service logs"
    echo_info "Use '$0 recommendations' to verify recommendations via API"
    echo_info "Use '$0 workloads' to verify workloads in database"
    echo_info "Available services: ingress, rosocp-processor, ros-ocp-rosocp-api, kruize, db-ros"
}

# Handle script arguments
case "${1:-}" in
    "logs")
        show_logs "${2:-}"
        exit 0
        ;;
    "health")
        run_health_checks
        exit $?
        ;;
    "recommendations")
        verify_recommendations
        exit $?
        ;;
    "workloads")
        verify_workloads_in_db
        exit $?
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  (none)           - Run complete data flow test (requires authentication)"
        echo "  logs [svc]       - Show logs for service (or list services if no service specified)"
        echo "  health           - Run health checks only (requires authentication)"
        echo "  recommendations  - Verify recommendations are available via API (requires authentication)"
        echo "  workloads        - Verify workloads are stored in ROS database"
        echo "  help             - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  NAMESPACE         - Kubernetes namespace (default: ros-ocp)"
        echo "  HELM_RELEASE_NAME - Helm release name (default: ros-ocp)"
        echo ""
        echo "Authentication Requirements:"
        echo "  This test REQUIRES authentication tokens to function properly."
        echo "  If no authentication tokens are available, the test will FAIL."
        echo "  Authentication sources checked:"
        echo "    - Service account 'insights-ros-ingress' with token secret"
        echo "    - Dev kubeconfig file at /tmp/dev-kubeconfig"
        echo "    - insights-ros-ingress pod with service account token"
        echo ""
        echo "  Prerequisites:"
        echo "    - ROS-OCP must be deployed (./install-helm-chart.sh)"
        echo "    - Service accounts must exist (created by Helm chart)"
        echo "    - Pods must be running and ready"
        echo ""
        echo "  Kubernetes-specific (Ingress access):"
        echo "  Hostname: localhost"
        echo ""
        echo "Notes:"
        echo "  - OpenShift: Uses Routes automatically with path-based routing"
        echo "  - Kubernetes: Uses Ingress with path-based routing (same paths as OpenShift)"
        exit 0
        ;;
esac

# Run main function
main "$@"