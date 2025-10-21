#!/bin/bash

# ROS-OCP OpenShift Data Flow Test Script
# This script tests the complete data flow in an OpenShift deployment using Routes

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
KAFKA_NAMESPACE=${KAFKA_NAMESPACE:-kafka}

# Port-forward configuration
USE_PORT_FORWARD=false
PORT_FORWARD_PIDS=()
CLEANUP_DONE=false

# Function to get port mapping for service
get_service_port() {
    local service_name="$1"
    case "$service_name" in
        "ingress") echo "3000" ;;
        "main") echo "8001" ;;  # ROS-OCP API (consolidated rosocp-api and main)
        "kruize") echo "8080" ;;
        *) echo "" ;;
    esac
}

# Cross-platform date function that works with both GNU and BSD date
# Usage: cross_platform_date_ago <minutes_ago> [format]
cross_platform_date_ago() {
    local minutes_ago="$1"
    local format="${2:-+%Y-%m-%d %H:%M:%S -0000 UTC}"
    local seconds_ago=$((minutes_ago * 60))
    local target_epoch=$(($(date +%s) - seconds_ago))

    # Try BSD date format first (macOS)
    if date -r "$target_epoch" "$format" 2>/dev/null; then
        return 0
    # Try GNU date format (Linux)
    elif date -d "@$target_epoch" "$format" 2>/dev/null; then
        return 0
    else
        # Fallback: use epoch time directly
        echo "$target_epoch"
        return 1
    fi
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

# Function to cleanup port-forwards
cleanup_port_forwards() {
    if [ "$CLEANUP_DONE" = "true" ]; then
        return 0
    fi

    if [ ${#PORT_FORWARD_PIDS[@]} -gt 0 ]; then
        echo_info "Cleaning up port-forwards..."
        for pid in "${PORT_FORWARD_PIDS[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                echo_info "Stopping port-forward (PID: $pid)"
                kill "$pid" 2>/dev/null || true
            fi
        done
        PORT_FORWARD_PIDS=()
        # Give processes time to cleanup
        sleep 2
        echo_success "Port-forward cleanup completed"
    fi
    CLEANUP_DONE=true
}

# Function to setup port-forwards for all services
setup_port_forwards() {
    echo_info "Setting up port-forwards for restricted environment..."

    local failed_forwards=0
    # Only set up port-forwards for services that aren't accessible via routes
    local services_to_forward=()
    for service in "ingress" "main" "kruize"; do
        if [[ ! " ${ACCESSIBLE_ROUTES:-} " =~ " $service " ]]; then
            services_to_forward+=("$service")
        fi
    done

    if [ ${#services_to_forward[@]} -eq 0 ]; then
        echo_info "All services accessible via routes - no port-forwarding needed"
        return 0
    fi

    echo_info "Setting up port-forwards for inaccessible services: ${services_to_forward[*]}"

    for service in "${services_to_forward[@]}"; do
        local local_port=$(get_service_port "$service")

        # Map service names correctly for OpenShift deployment
        local service_name
        local service_port
        case "$service" in
            "ingress")
                service_name="$HELM_RELEASE_NAME-ingress"
                service_port="8080"
                ;;
            "main")
                service_name="$HELM_RELEASE_NAME-rosocp-api"
                service_port="8000"
                ;;
            "kruize")
                service_name="$HELM_RELEASE_NAME-kruize"
                service_port="8080"
                ;;
            *)
                echo_warning "Unknown service: $service, skipping"
                continue
                ;;
        esac

        echo_info "Setting up port-forward: $service_name:$service_port -> localhost:$local_port"

        # Start port-forward in background
        oc port-forward -n "$NAMESPACE" "svc/$service_name" "$local_port:$service_port" >/dev/null 2>&1 &
        local pf_pid=$!

        # Wait a moment and check if port-forward started successfully
        sleep 2
        if kill -0 "$pf_pid" 2>/dev/null; then
            PORT_FORWARD_PIDS+=("$pf_pid")
            echo_success "Port-forward active: localhost:$local_port -> $service_name:$service_port (PID: $pf_pid)"
        else
            echo_error "Failed to start port-forward for $service"
            failed_forwards=$((failed_forwards + 1))
        fi
    done

    if [ $failed_forwards -gt 0 ]; then
        echo_warning "$failed_forwards port-forward(s) failed to start"
        echo_info "Some services may not be accessible"
    else
        echo_success "All port-forwards established successfully"
    fi

    # Wait a moment for all port-forwards to be fully established
    echo_info "Waiting for port-forwards to stabilize..."
    sleep 8
}

# Function to test port-forward connectivity
test_port_forward_connectivity() {
    echo_info "Testing port-forward connectivity..."
    local failed_tests=0

    # Only test services that should have port-forwards (not accessible via routes)
    local services_to_test=()
    for service in "ingress" "main" "kruize"; do
        if [[ ! " ${ACCESSIBLE_ROUTES:-} " =~ " $service " ]]; then
            services_to_test+=("$service")
        fi
    done

    if [ ${#services_to_test[@]} -eq 0 ]; then
        echo_success "No port-forwards to test - all services accessible via routes"
        return 0
    fi

    for service in "${services_to_test[@]}"; do
        local local_port=$(get_service_port "$service")
        local test_path

        case "$service" in
            "ingress")
                test_path="/ready"
                ;;
            "main")
                test_path="/status"
                ;;
            "kruize")
                test_path="/listPerformanceProfiles"
                ;;
            *)
                continue
                ;;
        esac

        # Try with longer timeout and retry logic for port-forwards to stabilize
        local max_attempts=3
        local attempt=1
        local service_accessible=false

        while [ $attempt -le $max_attempts ]; do
            if curl -s -f --connect-timeout 5 --max-time 10 "http://localhost:$local_port$test_path" >/dev/null 2>&1; then
                service_accessible=true
                break
            fi
            if [ $attempt -lt $max_attempts ]; then
                echo_info "  Attempt $attempt failed for $service, retrying in 2 seconds..."
                sleep 2
            fi
            attempt=$((attempt + 1))
        done

        if [ "$service_accessible" = true ]; then
            echo_success "$service accessible via localhost:$local_port"
        else
            echo_error "$service not accessible via localhost:$local_port (tried $max_attempts times)"
            failed_tests=$((failed_tests + 1))
        fi
    done

    if [ $failed_tests -eq 0 ]; then
        echo_success "All port-forward connections are working"
        return 0
    else
        echo_warning "$failed_tests service(s) not accessible via port-forward"
        return 1
    fi
}

# Function to check if oc/kubectl is configured for OpenShift
check_openshift() {
    # Prefer oc if available, fallback to kubectl
    local cmd="oc"
    if ! command -v oc >/dev/null 2>&1; then
        echo_warning "oc command not found, using kubectl"
        cmd="kubectl"
    fi

    if ! $cmd cluster-info >/dev/null 2>&1; then
        echo_error "OpenShift cluster is not accessible"
        return 1
    fi

    # Verify this is actually OpenShift
    if ! $cmd api-resources --api-group=route.openshift.io >/dev/null 2>&1; then
        echo_error "This does not appear to be an OpenShift cluster (no route.openshift.io API found)"
        return 1
    fi

    echo_success "Connected to OpenShift cluster"
    return 0
}

# Function to check if deployment exists
check_deployment() {
    if ! oc get namespace "$NAMESPACE" >/dev/null 2>&1; then
        echo_error "Namespace '$NAMESPACE' does not exist"
        return 1
    fi

    if ! helm list -n "$NAMESPACE" | grep -q "$HELM_RELEASE_NAME"; then
        echo_error "Helm release '$HELM_RELEASE_NAME' not found in namespace '$NAMESPACE'"
        return 1
    fi

    return 0
}

# Function to test if a URL is accessible
test_url_accessible() {
    local url="$1"
    curl -s -f --connect-timeout 5 "$url" >/dev/null 2>&1
}

# Function to test route accessibility for all essential routes
test_route_accessibility() {
    echo_info "Testing accessibility of essential routes..."

    local accessible_routes=()
    local inaccessible_routes=()
    local essential_routes=("main" "ingress" "kruize")

    for route_service in "${essential_routes[@]}"; do
        local route_host=$(oc get route "$HELM_RELEASE_NAME-$route_service" -n "$NAMESPACE" -o jsonpath='{.spec.host}' 2>/dev/null)
        if [ -n "$route_host" ]; then
            local test_url="http://$route_host/"
            if [ "$route_service" = "main" ]; then
                test_url="http://$route_host/status"
            elif [ "$route_service" = "ingress" ]; then
                test_url="http://$route_host/api/ingress/v1/version"
            elif [ "$route_service" = "kruize" ]; then
                test_url="http://$route_host/listPerformanceProfiles"
            fi

            if test_url_accessible "$test_url"; then
                accessible_routes+=("$route_service")
                echo_success "✓ $route_service route accessible: $route_host"
            else
                inaccessible_routes+=("$route_service")
                echo_warning "✗ $route_service route not accessible: $route_host"
            fi
        else
            inaccessible_routes+=("$route_service")
            echo_warning "✗ $route_service route not found"
        fi
    done

    echo_info "Route accessibility summary:"
    echo_info "  Accessible: ${#accessible_routes[@]}/3 routes (${accessible_routes[*]})"
    echo_info "  Inaccessible: ${#inaccessible_routes[@]}/3 routes (${inaccessible_routes[*]})"

    # Store results for use by get_service_url
    export ACCESSIBLE_ROUTES="${accessible_routes[*]}"
    export INACCESSIBLE_ROUTES="${inaccessible_routes[*]}"

    # Determine access method
    if [ ${#accessible_routes[@]} -eq 3 ]; then
        echo_success "All routes are externally accessible - using direct route access"
        return 0
    elif [ ${#accessible_routes[@]} -gt 0 ]; then
        echo_info "Mixed accessibility - will use routes where possible, port-forwarding for others"
        return 1
    else
        echo_warning "No routes are externally accessible - will use port-forwarding"
        return 2
    fi
}

# Function to get service URL using OpenShift Routes or port-forward (hybrid approach)
get_service_url() {
    local service_name="$1"
    local path="$2"

    # Check if this specific route is accessible (from test_route_accessibility results)
    local route_accessible=false
    if [[ " ${ACCESSIBLE_ROUTES:-} " =~ " $service_name " ]]; then
        route_accessible=true
    fi

    # If route is accessible, use it directly
    if [ "$route_accessible" = true ]; then
        local route_name="$HELM_RELEASE_NAME-$service_name"
        local route_host=$(oc get route "$route_name" -n "$NAMESPACE" -o jsonpath='{.spec.host}' 2>/dev/null)
        if [ -n "$route_host" ]; then
            # Check if route uses TLS
            local tls_termination=$(oc get route "$route_name" -n "$NAMESPACE" -o jsonpath='{.spec.tls.termination}' 2>/dev/null)
            if [ -n "$tls_termination" ]; then
                echo "https://$route_host$path"
            else
                echo "http://$route_host$path"
            fi
        else
            echo_error "Route $route_name not found in namespace $NAMESPACE" >&2
            return 1
            return 0
        fi
    fi

    # Fall back to port-forward for this specific service
    local local_port=$(get_service_port "$service_name")
    if [ -n "$local_port" ]; then
        echo "http://localhost:$local_port$path"
        return 0
    else
        echo_error "No port mapping found for service: $service_name and route not accessible"
        return 1
    fi
}

# Function to wait for services to be ready
wait_for_services() {
    echo_info "Waiting for OpenShift services to be ready..."

    local retries=60
    local count=0
    local required_routes=("main" "ingress" "kruize")

    while [ $count -lt $retries ]; do
        local ready_routes=0

        for route_service in "${required_routes[@]}"; do
            local route_name="$HELM_RELEASE_NAME-$route_service"
            if oc get route "$route_name" -n "$NAMESPACE" >/dev/null 2>&1; then
                # Check if route is admitted
                local admitted=$(oc get route "$route_name" -n "$NAMESPACE" -o jsonpath='{.status.ingress[0].conditions[?(@.type=="Admitted")].status}' 2>/dev/null)
                if [ "$admitted" = "True" ]; then
                    ready_routes=$((ready_routes + 1))
                fi
            fi
        done

        if [ $ready_routes -eq ${#required_routes[@]} ]; then
            echo_success "All required routes are ready"
            return 0
        fi

        if [ $((count % 10)) -eq 0 ]; then
            echo_info "Waiting for routes to be ready... ($ready_routes/${#required_routes[@]} ready)"
        fi

        sleep 5
        count=$((count + 1))
    done

    echo_error "Timeout waiting for routes to be ready"
    echo_info "Current route status:"
    oc get routes -n "$NAMESPACE" -o wide
    return 1
}

# Function to create test data
create_test_data() {
    echo_info "Creating test data with current timestamps..." >&2

    # Generate dynamic timestamps for current data (multiple intervals for better recommendations)
    # Use cross-platform date function
    local now_date=$(date -u +%Y-%m-%d)
    local interval_start_1=$(cross_platform_date_ago 75)  # 75 minutes ago
    local interval_end_1=$(cross_platform_date_ago 60)    # 60 minutes ago
    local interval_start_2=$(cross_platform_date_ago 60)  # 60 minutes ago
    local interval_end_2=$(cross_platform_date_ago 45)    # 45 minutes ago
    local interval_start_3=$(cross_platform_date_ago 45)  # 45 minutes ago
    local interval_end_3=$(cross_platform_date_ago 30)    # 30 minutes ago
    local interval_start_4=$(cross_platform_date_ago 30)  # 30 minutes ago
    local interval_end_4=$(cross_platform_date_ago 15)    # 15 minutes ago

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

# Function to upload test data
upload_test_data() {
    echo_info "=== STEP 1: Upload Test Data ===="

    local test_csv=$(create_test_data)
    local test_dir=$(mktemp -d)
    local csv_filename="openshift_usage_report.csv"
    local manifest_filename="manifest.json"
    local tar_filename="cost-mgmt.tar.gz"

    # Copy CSV to temporary directory with expected filename
    if ! cp "$test_csv" "$test_dir/$csv_filename"; then
        echo_error "Failed to copy CSV file to temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Create required manifest.json file for OpenShift ingress service
    local file_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local cluster_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local current_date=$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")
    local start_date=$(date -u +"%Y-%m-%dT%H:00:00Z")
    local end_date=$(date -u +"%Y-%m-%dT%H:59:59Z")
    
    cat > "$test_dir/$manifest_filename" << EOF
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
    "end": "$end_date"
}
EOF

    # Verify the files exist and have content
    if [ ! -f "$test_dir/$csv_filename" ] || [ ! -s "$test_dir/$csv_filename" ]; then
        echo_error "CSV file not found or is empty in temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    if [ ! -f "$test_dir/$manifest_filename" ] || [ ! -s "$test_dir/$manifest_filename" ]; then
        echo_error "Manifest file not found or is empty in temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Create tar.gz file
    echo_info "Creating tar.gz archive..."
    if ! (cd "$test_dir" && tar -czf "$tar_filename" "$csv_filename" "$manifest_filename"); then
        echo_error "Failed to create tar.gz archive"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Verify tar.gz file was created
    if [ ! -f "$test_dir/$tar_filename" ]; then
        echo_error "tar.gz file was not created"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    echo_info "Uploading tar.gz file..."

    # Upload the tar.gz file using curl with proper headers and content-type
    # In OpenShift, file upload is handled via the ingress route
    local upload_url=$(get_service_url "ingress" "/api/ingress/v1/upload")
    echo_info "Uploading to: $upload_url"

    # Get service account token for authentication (OpenShift approach)
    local sa_token
    if kubectl get secret -n "$NAMESPACE" | grep -q "ros-ocp-backend-token"; then
        # Use existing service account token
        sa_token=$(kubectl get secret -n "$NAMESPACE" -o jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="ros-ocp-backend")].data.token}' | head -1 | base64 -d)
    else
        # Create a service account token for testing
        echo_info "Creating service account token for authentication..."
        kubectl create token ros-ocp-backend -n "$NAMESPACE" --duration=3600s > /tmp/sa-token 2>/dev/null || {
            echo_warning "Could not create service account token, using x-rh-identity only"
            sa_token=""
        }
        if [ -f /tmp/sa-token ]; then
            sa_token=$(cat /tmp/sa-token)
            rm -f /tmp/sa-token
        fi
    fi

    # Build curl command with proper authentication
    local curl_cmd="curl -s -w \"%{http_code}\" --connect-timeout 10 --max-time 60"
    curl_cmd="$curl_cmd -F \"file=@${test_dir}/${tar_filename};type=application/vnd.redhat.hccm.filename+tgz\""
    
    # Add authentication headers
    if [ -n "$sa_token" ]; then
        curl_cmd="$curl_cmd -H \"Authorization: Bearer $sa_token\""
        echo_info "Using OpenShift service account token for authentication"
    else
        echo_info "Using x-rh-identity header for authentication"
    fi
    
    curl_cmd="$curl_cmd -H \"x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K\""
    curl_cmd="$curl_cmd -H \"x-rh-request-id: test-request-$(date +%s)\""
    curl_cmd="$curl_cmd \"$upload_url\""

    local response=$(eval $curl_cmd)

    local http_code="${response: -3}"
    local response_body="${response%???}"

    # Cleanup
    rm -f "$test_csv"
    rm -rf "$test_dir"

    if [ "$http_code" != "202" ]; then
        echo_error "Upload failed with HTTP $http_code"
        echo_error "Response: $response_body"
        return 1
    fi

    echo_success "Upload successful! HTTP $http_code"
    echo_info "Response: $response_body"

    return 0
}

# Function to check ODF credentials secret
check_odf_credentials() {
    # Check for the ODF credentials secret
    local odf_credentials_secret=$(oc get secret -n "$NAMESPACE" "ros-ocp-odf-credentials" -o name 2>/dev/null)
    
    if [ -z "$odf_credentials_secret" ]; then
        echo_error "ODF credentials secret 'ros-ocp-odf-credentials' not found in namespace '$NAMESPACE'"
        echo_error "Please create this secret with your ODF S3 credentials:"
        echo_error "  oc create secret generic ros-ocp-odf-credentials \\"
        echo_error "    --namespace=$NAMESPACE \\"
        echo_error "    --from-literal=access-key=<access-key> \\"
        echo_error "    --from-literal=secret-key=<secret-key>"
        return 1
    fi
    
    return 0
}

# Function to get ODF storage endpoint
get_storage_endpoint() {
    # Dynamic ODF S3 service discovery using NooBaa CRD status
    echo_info "Querying NooBaa CRD for S3 endpoint..."
    local s3_endpoint=$(oc get noobaas.noobaa.io -n openshift-storage -o jsonpath='{.items[0].status.services.serviceS3.internalDNS[0]}' 2>/dev/null)
    echo_info "Raw NooBaa response: '$s3_endpoint'"
    
    if [ -n "$s3_endpoint" ]; then
        # Clean up the endpoint
        s3_endpoint=$(echo "$s3_endpoint" | sed 's|https://||' | sed 's|:443||')
        echo_info "Cleaned endpoint: '$s3_endpoint'"
        
        # Convert to full cluster DNS if needed
        if [[ ! "$s3_endpoint" =~ \.cluster\.local$ ]]; then
            s3_endpoint="${s3_endpoint}.cluster.local"
        fi
        echo_info "Final endpoint: '$s3_endpoint:443'"
        echo "$s3_endpoint:443"
    else
        echo_warning "NooBaa CRD query returned empty result, using fallback endpoint"
        echo_info "Using fallback ODF S3 endpoint: s3.openshift-storage.svc.cluster.local:443"
        echo "s3.openshift-storage.svc.cluster.local:443"
    fi
}

# Function to get ODF storage credentials
get_storage_credentials() {
    # Get credentials from our ODF credentials secret
    local access_key=$(oc get secret -n "$NAMESPACE" ros-ocp-odf-credentials -o jsonpath='{.data.access-key}' | base64 -d 2>/dev/null)
    local secret_key=$(oc get secret -n "$NAMESPACE" ros-ocp-odf-credentials -o jsonpath='{.data.secret-key}' | base64 -d 2>/dev/null)
    
    if [ -n "$access_key" ] && [ -n "$secret_key" ]; then
        echo "$access_key:$secret_key"
    else
        echo_error "Failed to get ODF credentials from ros-ocp-odf-credentials secret"
        return 1
    fi
}

# Function to simulate Koku processing
simulate_koku_processing() {
    echo_info "=== STEP 2: Simulate Koku Processing ===="

    # Check ODF credentials
    if ! check_odf_credentials; then
        return 1
    fi

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
    # Use cross-platform date function
    local now_date=$(date -u +%Y-%m-%d)
    local interval_start_1=$(cross_platform_date_ago 60)  # 60 minutes ago
    local interval_end_1=$(cross_platform_date_ago 45)    # 45 minutes ago
    local interval_start_2=$(cross_platform_date_ago 45)  # 45 minutes ago
    local interval_end_2=$(cross_platform_date_ago 30)    # 30 minutes ago
    local interval_start_3=$(cross_platform_date_ago 30)  # 30 minutes ago
    local interval_end_3=$(cross_platform_date_ago 15)    # 15 minutes ago

    echo_info "Creating CSV with current timestamps:" >&2
    echo_info "  Report date: $now_date" >&2
    echo_info "  Multiple intervals for better recommendations" >&2

    local csv_content="report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
$now_date,$now_date,$interval_start_1,$interval_end_1,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.247832,0.185671,0.324131,0.247832,0.001,0.002,0.001,536870912,536870912,1073741824,1073741824,413587266.064516,410009344,420900544,413587266.064516,393311537.548387,390293568,396371392,393311537.548387
$now_date,$now_date,$interval_start_2,$interval_end_2,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.265423,0.198765,0.345678,0.265423,0.0012,0.0025,0.0012,536870912,536870912,1073741824,1073741824,427891456.123456,422014016,435890624,427891456.123456,407654321.987654,403627568,411681024,407654321.987654
$now_date,$now_date,$interval_start_3,$interval_end_3,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,0.5,0.5,1.0,1.0,0.289567,0.210987,0.367890,0.289567,0.0008,0.0018,0.0008,536870912,536870912,1073741824,1073741824,445678901.234567,441801728,449556074,445678901.234567,425987654.321098,421960800,430014256,425987654.321098"

    # Get storage endpoint and credentials
    local storage_endpoint
    storage_endpoint=$(get_storage_endpoint)
    if [ $? -ne 0 ]; then
        echo_error "Failed to get storage endpoint"
        return 1
    fi

    local storage_credentials
    storage_credentials=$(get_storage_credentials)
    if [ $? -ne 0 ]; then
        echo_error "Failed to get storage credentials"
        return 1
    fi

    local access_key="${storage_credentials%:*}"
    local secret_key="${storage_credentials#*:}"

    echo_info "Storage endpoint: $storage_endpoint"
    echo_info "Copying CSV to ros-data bucket..."

    # ODF approach - use ingress service to upload file
    echo_info "Using ingress service to upload CSV to ODF bucket..."
    
    # Create a temporary directory and CSV file
    local temp_dir=$(mktemp -d)
    local temp_csv="${temp_dir}/${csv_filename}"
    echo "$csv_content" > "$temp_csv"
    
    # Create required manifest.json file for OpenShift ingress service
    # Unlike MinIO (direct S3 API), ODF requires the ingress service as a gateway
    # The ingress service expects a tar.gz archive containing:
    # 1. The CSV file with usage data
    # 2. A manifest.json file describing the payload structure
    # This manifest tells the ingress service how to process and route the files to ODF S3
    local file_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local cluster_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local current_date=$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")
    local start_date=$(date -u +"%Y-%m-%dT%H:00:00Z")
    local end_date=$(date -u +"%Y-%m-%dT%H:59:59Z")
    
    cat > "${temp_dir}/manifest.json" << EOF
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
    "end": "$end_date"
}
EOF
    
    # Create tar.gz archive (same format as real uploads)
    local temp_tar="${temp_dir}/cost-mgmt.tar.gz"
    tar -czf "$temp_tar" -C "$temp_dir" "$csv_filename" "manifest.json"

    # Upload via ingress service (which handles ODF S3)
    local upload_url=$(get_service_url "ingress" "/api/ingress/v1/upload")
    echo_info "Uploading CSV to: $upload_url"

    # Get service account token for authentication
    local sa_token
    if kubectl get secret -n "$NAMESPACE" | grep -q "ros-ocp-backend-token"; then
        sa_token=$(kubectl get secret -n "$NAMESPACE" -o jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="ros-ocp-backend")].data.token}' | head -1 | base64 -d)
    else
        kubectl create token ros-ocp-backend -n "$NAMESPACE" --duration=3600s > /tmp/sa-token 2>/dev/null || {
            echo_warning "Could not create service account token, using x-rh-identity only"
            sa_token=""
        }
        if [ -f /tmp/sa-token ]; then
            sa_token=$(cat /tmp/sa-token)
            rm -f /tmp/sa-token
        fi
    fi

    # Build curl command for CSV upload
    local curl_cmd="curl -s -w \"%{http_code}\" --connect-timeout 10 --max-time 60"
    curl_cmd="$curl_cmd -F \"file=@${temp_tar};type=application/vnd.redhat.hccm.filename+tgz\""
    
    if [ -n "$sa_token" ]; then
        curl_cmd="$curl_cmd -H \"Authorization: Bearer $sa_token\""
    fi
    
    curl_cmd="$curl_cmd -H \"x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K\""
    curl_cmd="$curl_cmd -H \"x-rh-request-id: test-csv-$(date +%s)\""
    curl_cmd="$curl_cmd \"$upload_url\""

    local response=$(eval $curl_cmd)
    local http_code="${response: -3}"
    local response_body="${response%???}"

    # Cleanup
    rm -rf "$temp_dir"

    if [ "$http_code" != "202" ]; then
        echo_error "CSV upload failed with HTTP $http_code"
        echo_error "Response: $response_body"
        return 1
    fi

    echo_success "CSV file uploaded to ODF bucket via ingress service"
    echo_info "Response: $response_body"

    echo_info "=== STEP 3: Publish Kafka Event ===="

    # Create Kafka message with ODF storage URL
    # For ODF, use the ingress service URL since files are uploaded via ingress
    local storage_url="http://${HELM_RELEASE_NAME}-ingress:8080/ros-data/$csv_filename"

    local kafka_message="{\"request_id\":\"test-request-$(date +%s)\",\"b64_identity\":\"eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K\",\"metadata\":{\"account\":\"12345\",\"org_id\":\"12345\",\"source_id\":\"test-source-id\",\"cluster_uuid\":\"1b77b73f-1d3e-43c6-9f55-bcd9fb6d1a0c\",\"cluster_alias\":\"test-cluster\"},\"files\":[\"$storage_url\"]}"

    echo_info "Publishing Kafka message to hccm.ros.events topic"
    echo_info "Message content: $kafka_message"

    # Get Kafka pod
    local kafka_pod=$(oc get pods -n "$KAFKA_NAMESPACE" -l "app.kubernetes.io/name=kafka" -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$kafka_pod" ]; then
        echo_error "Kafka pod not found"
        return 1
    fi

    # Publish message to Kafka
    echo "$kafka_message" | oc exec -i -n "$KAFKA_NAMESPACE" "$kafka_pod" -- \
        /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server ros-ocp-kafka-kafka-bootstrap:9092 --topic hccm.ros.events

    if [ $? -eq 0 ]; then
        echo_success "Kafka message published successfully"
        echo_info "File UUID: $file_uuid"
        echo_info "CSV file: $csv_filename"
        echo_info "Accessible via ODF S3 service (internal access only)"
    else
        echo_error "Failed to publish Kafka message"
        return 1
    fi

    # Wait for processing
    echo_info "Waiting for message processing (30 seconds)..."
    sleep 30

    return 0
}

# Function to verify processing
verify_processing() {
    echo_info "=== STEP 4: Verify Processing ===="

    # Check if processor pod received the message
    echo_info "Checking processor pod logs for recent activity..."
    local processor_pod=$(oc get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rosocp-processor" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$processor_pod" ]; then
        echo_info "Recent processor logs:"
        oc logs -n "$NAMESPACE" "$processor_pod" --tail=20 | grep -i "processing\|error\|complete" || echo "No relevant processing messages found"
    else
        echo_warning "Processor pod not found"
    fi

    # Check Kafka topic for messages
    echo_info "Checking Kafka topics..."
    local kafka_pod=$(oc get pods -n "$KAFKA_NAMESPACE" -l "app.kubernetes.io/name=kafka" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$kafka_pod" ]; then
        echo_info "Messages in hccm.ros.events topic:"
        local event_messages=$(oc exec -n "$KAFKA_NAMESPACE" "$kafka_pod" -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server ros-ocp-kafka-kafka-bootstrap:9092 --topic hccm.ros.events --from-beginning --timeout-ms 5000 2>/dev/null | wc -l || echo "0")
        echo_info "  Event messages: $event_messages"

        echo_info "Messages in rosocp.kruize.recommendations topic:"
        local rec_messages=$(oc exec -n "$KAFKA_NAMESPACE" "$kafka_pod" -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server ros-ocp-kafka-kafka-bootstrap:9092 --topic rosocp.kruize.recommendations --from-beginning --timeout-ms 5000 2>/dev/null | wc -l || echo "0")
        echo_info "  Recommendation messages: $rec_messages"
    fi

    echo_info "Processing verification completed"
}

# Function to verify recommendations
verify_recommendations() {
    echo_info "=== STEP 5: Verify Recommendations ===="

    # Verify Kruize API is accessible
    local kruize_url=$(get_service_url "kruize" "/listPerformanceProfiles")
    echo_info "Checking Kruize API accessibility at: $kruize_url"

    if curl -f -s --connect-timeout 5 --max-time 15 "$kruize_url" >/dev/null; then
        echo_success "✓ Kruize API is accessible"
    else
        echo_error "Kruize API is not accessible"
        return 1
    fi

    # Check for actual ML recommendations in database
    echo_info "Checking for ML recommendations in database..."
    local db_pod=$(oc get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=db-ros" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$db_pod" ]; then
        local rec_count=$(oc exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c \
            "SELECT COUNT(*) FROM recommendation_sets;" 2>/dev/null | tr -d ' ' || echo "0")

        if [ "$rec_count" -gt 0 ]; then
            echo_success "✓ Found $rec_count ML recommendation(s) generated by Kruize"

            # Show actual recommendation details
            echo_info "Latest ML recommendation summary:"
            oc exec -n "$NAMESPACE" "$db_pod" -- \
                psql -U postgres -d postgres -c \
                "SELECT
                    container_name,
                    (recommendations->'current'->'requests'->'cpu'->>'amount')::float as current_cpu_cores,
                    (recommendations->'recommendation_terms'->'short_term'->'recommendation_engines'->'cost'->'config'->'requests'->'cpu'->>'amount')::float as recommended_cpu_cores,
                    round((recommendations->'current'->'requests'->'memory'->>'amount')::float / 1024.0 / 1024.0) as current_memory_mb,
                    round((recommendations->'recommendation_terms'->'short_term'->'recommendation_engines'->'cost'->'config'->'requests'->'memory'->>'amount')::float / 1024.0 / 1024.0) as recommended_memory_mb
                 FROM recommendation_sets
                 ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "Could not retrieve recommendation details"

            echo_success "✓ ML recommendations successfully generated and saved"
        else
            echo_warning "No ML recommendations found in database"
            echo_info "This may indicate:"
            echo_info "  - Recommendations are still being processed"
            echo_info "  - Insufficient data for ML analysis"
        fi
    else
        echo_warning "Database pod not found - cannot verify recommendations"
    fi

    # Check recommendation poller logs
    echo_info "Checking recommendation poller activity..."
    local poller_pod=$(oc get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rosocp-recommendation-poller" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$poller_pod" ]; then
        echo_info "Recent poller logs:"
        oc logs -n "$NAMESPACE" "$poller_pod" --tail=10 | grep -i "recommendation\|poll\|error" || echo "No relevant poller messages found"
    else
        echo_warning "Recommendation poller pod not found"
    fi

    return 0
}

# Function to verify workloads in database
verify_workloads_in_db() {
    echo_info "=== STEP 6: Verify Workloads in Database ===="

    local db_pod=$(oc get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=db-ros" -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$db_pod" ]; then
        echo_error "Database pod not found"
        return 1
    fi

    echo_info "Checking workloads table in ROS database..."

    # Check if workloads table has data
    local workload_count=$(oc exec -n "$NAMESPACE" "$db_pod" -- \
        psql -U postgres -d postgres -t -c \
        "SELECT COUNT(*) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")

    if [ "$workload_count" -gt 0 ]; then
        echo_success "✓ Found $workload_count workload(s) in database"

        # Get workload details
        echo_info "Workload summary:"
        oc exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -c \
            "SELECT workload_name, namespace, workload_type, metrics_upload_at
             FROM workloads
             ORDER BY metrics_upload_at DESC
             LIMIT 5;" 2>/dev/null || echo "Could not retrieve workload details"

        # Check for clusters
        local cluster_count=$(oc exec -n "$NAMESPACE" "$db_pod" -- \
            psql -U postgres -d postgres -t -c \
            "SELECT COUNT(DISTINCT cluster_id) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")

        echo_info "  Workloads span $cluster_count cluster(s)"

    else
        echo_warning "No workload data found in database"
        echo_info "This might indicate:"
        echo_info "  - Data processing is still in progress"
        echo_info "  - No data has been uploaded yet"
        echo_info "  - There was an issue with data processing"
    fi

    echo_info "Workload database verification completed"
}

# Function to run health checks using OpenShift Routes (only for essential routes)
run_health_checks() {
    echo_info "=== Health Checks ===="

    local failed_checks=0

    echo_info "Running health checks for OpenShift deployment"

    # List all available routes
    echo_info "Available routes:"
    oc get routes -n "$NAMESPACE" -o custom-columns=NAME:.metadata.name,HOST:.spec.host,PATH:.spec.path

    # Check each essential service via its route (only the 2 remaining routes)
    local services=("main" "kruize")
    local paths=("/status" "/listPerformanceProfiles")
    local route_names=("ros-ocp-main" "ros-ocp-kruize")

    for i in "${!services[@]}"; do
        local service="${services[$i]}"
        local path="${paths[$i]}"
        local url=$(get_service_url "$service" "$path")

        if [ $? -eq 0 ]; then
            if curl -f -s --connect-timeout 5 --max-time 15 "$url" >/dev/null; then
                echo_success "$service is accessible at: $url"
            else
                echo_error "$service is not accessible at: $url"
                failed_checks=$((failed_checks + 1))
            fi
        else
            echo_error "Could not determine URL for $service"
            failed_checks=$((failed_checks + 1))
        fi
    done

    # Check pod status
    local pending_pods=$(oc get pods -n "$NAMESPACE" --field-selector=status.phase=Pending --no-headers 2>/dev/null | wc -l)
    local failed_pods=$(oc get pods -n "$NAMESPACE" --field-selector=status.phase=Failed --no-headers 2>/dev/null | wc -l)

    if [ "$pending_pods" -eq 0 ] && [ "$failed_pods" -eq 0 ]; then
        echo_success "All pods are running successfully"
    else
        echo_warning "$pending_pods pending pods, $failed_pods failed pods"
        failed_checks=$((failed_checks + 1))
    fi

    # Check route status
    echo_info "Checking route status..."
    local total_routes=$(oc get routes -n "$NAMESPACE" --no-headers | wc -l)
    local admitted_routes=$(oc get routes -n "$NAMESPACE" -o jsonpath='{range .items[*]}{.status.ingress[0].conditions[?(@.type=="Admitted")].status}{"\n"}{end}' | grep -c "True" || echo "0")

    echo_info "Routes: $admitted_routes/$total_routes admitted"
    if [ "$admitted_routes" -ne "$total_routes" ]; then
        echo_warning "Not all routes are admitted"
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
        oc get pods -n "$NAMESPACE" -o custom-columns="NAME:.metadata.name,COMPONENT:.metadata.labels.app\.kubernetes\.io/name" --no-headers
        return 0
    fi

    local pod=$(oc get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$service" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -n "$pod" ]; then
        echo_info "Logs for $service ($pod):"
        oc logs -n "$NAMESPACE" "$pod" --tail=50
    else
        echo_error "Pod not found for service: $service"
        return 1
    fi
}

# Main execution
main() {
    echo_info "ROS-OCP OpenShift Data Flow Test"
    echo_info "================================"

    # Check prerequisites
    if ! check_openshift; then
        exit 1
    fi

    if ! check_deployment; then
        exit 1
    fi

    # Setup cleanup trap
    trap cleanup_port_forwards EXIT INT TERM

    echo_info "Configuration:"
    echo_info "  Platform: OpenShift"
    echo_info "  Namespace: $NAMESPACE"
    echo_info "  Helm Release: $HELM_RELEASE_NAME"

    # Test route accessibility for all essential services
    local accessibility_result
    set +e  # Temporarily disable exit on error for the accessibility test
    test_route_accessibility
    accessibility_result=$?
    set -e  # Re-enable exit on error

    case $accessibility_result in
        0)
            # All routes accessible
            echo_info "  Access Method: OpenShift Routes (external access)"
            USE_PORT_FORWARD=false
            ;;
        1)
            # Mixed accessibility - use hybrid approach
            echo_info "  Access Method: Hybrid (routes + selective port-forwarding)"
            USE_PORT_FORWARD=false

            # Set up port-forwards only for inaccessible services
            if setup_port_forwards; then
                echo_success "Hybrid access setup completed successfully"
            else
                echo_warning "Some port-forwards failed, but accessible routes will still work"
            fi
            ;;
        2)
            # No routes accessible
            echo_info "This may be due to:"
            echo_info "  - Network restrictions (internal-only cluster)"
            echo_info "  - DNS resolution issues"
            echo_info "  - Firewall policies"
            echo_info ""
            echo_info "Setting up port-forwarding for all services..."
            USE_PORT_FORWARD=true

            # Force port-forwards for all services
            unset ACCESSIBLE_ROUTES
            export ACCESSIBLE_ROUTES=""

            if setup_port_forwards; then
                echo_success "Port-forward setup completed successfully"
                echo_info "  Access Method: Port-forward (localhost access)"
            else
                echo_error "Port-forward setup failed. Some services may not be accessible."
                echo_info "  Access Method: Port-forward (partial connectivity)"
            fi
            ;;
    esac
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

    if simulate_koku_processing; then
        echo_success "Steps 2-3: Koku simulation and Kafka event completed successfully"
    else
        echo_error "Steps 2-3: Koku simulation failed"
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
    echo_success "OpenShift data flow test completed!"
    echo_info "Use '$0 logs <service>' to view specific service logs"
    echo_info "Use '$0 recommendations' to verify recommendations via API"
    echo_info "Use '$0 workloads' to verify workloads in database"
    if [ "$USE_PORT_FORWARD" = "true" ]; then
        echo_info "Use '$0 cleanup' to stop port-forwards (or they'll stop automatically on exit)"
    fi
    echo_info "Available services: rosocp-processor, main (ROS-OCP API), kruize, database"
}

# Handle script arguments
case "${1:-}" in
    "logs")
        show_logs "${2:-}"
        exit 0
        ;;
    "health")
        # Setup cleanup trap for health checks
        trap cleanup_port_forwards EXIT INT TERM

        # Test route accessibility for health checks
        echo_info "Checking route accessibility for health checks..."
        set +e  # Temporarily disable exit on error
        test_route_accessibility >/dev/null
        health_accessibility_result=$?
        set -e  # Re-enable exit on error

        case $health_accessibility_result in
            0)
                echo_info "Using external routes for health checks"
                USE_PORT_FORWARD=false
                ;;
            1)
                echo_info "Using hybrid access for health checks"
                USE_PORT_FORWARD=false
                setup_port_forwards >/dev/null
                ;;
            2)
                echo_warning "Routes not externally accessible, setting up port-forwarding for health checks..."
                USE_PORT_FORWARD=true
                unset ACCESSIBLE_ROUTES
                export ACCESSIBLE_ROUTES=""
                setup_port_forwards >/dev/null
                echo_info "Port-forwarding enabled for health checks"
                ;;
        esac

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
    "cleanup")
        cleanup_port_forwards
        exit 0
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  (no command)    Run complete data flow test"
        echo "  logs [service]  Show logs for specific service (or list services)"
        echo "  health          Run health checks only"
        echo "  recommendations Verify recommendations via Kruize API"
        echo "  workloads       Verify workloads in database"
        echo "  cleanup         Clean up any active port-forwards"
        echo "  help            Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  NAMESPACE            Target namespace (default: ros-ocp)"
        echo "  HELM_RELEASE_NAME    Helm release name (default: ros-ocp)"
        echo "  KAFKA_NAMESPACE      Kafka namespace (default: kafka)"
        echo ""
        echo "OpenShift Requirements:"
        echo "  - oc or kubectl must be configured for OpenShift cluster"
        echo "  - OpenShift Routes must be available (route.openshift.io API)"
        echo "  - ROS-OCP must be deployed via Helm chart"
        echo ""
        echo "Network Access:"
        echo "  - Script automatically detects route accessibility"
        echo "  - If routes are externally accessible: uses OpenShift Routes"
        echo "  - If routes are not accessible: automatically sets up port-forwarding"
        echo "  - Port-forwards are cleaned up automatically on script exit"
        echo "  - Manual cleanup available with: $0 cleanup"
        echo ""
        echo "Port Mappings (when using port-forwarding):"
        echo "  - ingress:     localhost:3000  -> file upload service"
        echo "  - main:        localhost:8001  -> ROS-OCP API service"
        echo "  - kruize:      localhost:8080  -> ML recommendations"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        echo_error "Unknown command: $1"
        echo_info "Use '$0 help' for usage information"
        exit 1
        ;;
esac
