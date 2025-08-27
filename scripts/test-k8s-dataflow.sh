#!/bin/bash

# ROS-OCP Kubernetes Data Flow Test Script
# This script tests the complete data flow in a Kubernetes deployment

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
INGRESS_PORT=${INGRESS_PORT:-30080}
API_PORT=${API_PORT:-30081}
KRUIZE_PORT=${KRUIZE_PORT:-30090}

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

# Function to check if kubectl is configured
check_kubectl() {
    if ! kubectl cluster-info >/dev/null 2>&1; then
        echo_error "kubectl is not configured or cluster is not accessible"
        return 1
    fi
    return 0
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

# Function to wait for services to be ready
wait_for_services() {
    echo_info "Waiting for services to be ready..."

    # Wait for pods to be ready
    kubectl wait --for=condition=ready pod -l "app.kubernetes.io/instance=$HELM_RELEASE_NAME" \
        --namespace "$NAMESPACE" \
        --timeout=300s \
        --field-selector=status.phase!=Succeeded

    echo_success "All pods are ready"

    # Wait for services to be accessible
    local retries=30
    local count=0

    while [ $count -lt $retries ]; do
        if curl -f -s http://localhost:${INGRESS_PORT}/api/ingress/v1/version >/dev/null 2>&1; then
            echo_success "Ingress service is accessible"
            break
        fi

        echo_info "Waiting for ingress service to be accessible... ($((count + 1))/$retries)"
        sleep 10
        count=$((count + 1))
    done

    if [ $count -eq $retries ]; then
        echo_error "Ingress service is not accessible after $retries attempts"
        return 1
    fi
}

# Function to create test data
create_test_data() {
    echo_info "Creating test data..." >&2

    # Create a temporary CSV file with proper ROS-OCP format
    local test_csv=$(mktemp)
    cat > "$test_csv" << 'EOF'
report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
2024-01-01,2024-01-01,2024-01-01 00:00:00 -0000 UTC,2024-01-01 00:15:00 -0000 UTC,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,100,100,200,200,50,10,90,50,0,0,0,512,512,1024,1024,256,128,384,256,200,100,300,200
2024-01-01,2024-01-01,2024-01-01 00:15:00 -0000 UTC,2024-01-01 00:30:00 -0000 UTC,test-container-2,test-pod-456,test-deployment-2,Deployment,test-workload-2,deployment,test-namespace-2,quay.io/test/image2:latest,worker-node-2,resource-456,150,150,300,300,75,20,120,75,5,10,5,768,768,1536,1536,384,192,576,384,300,150,450,300
EOF

    echo "$test_csv"
}

# Function to upload test data
upload_test_data() {
    echo_info "=== STEP 1: Upload Test Data ===="

    local test_csv=$(create_test_data)
    local test_dir=$(mktemp -d)
    local csv_filename="openshift_usage_report.csv"
    local tar_filename="cost-mgmt.tar.gz"

    # Copy CSV to temporary directory with expected filename
    if ! cp "$test_csv" "$test_dir/$csv_filename"; then
        echo_error "Failed to copy CSV file to temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Verify the file exists and has content
    if [ ! -f "$test_dir/$csv_filename" ] || [ ! -s "$test_dir/$csv_filename" ]; then
        echo_error "CSV file not found or is empty in temporary directory"
        rm -f "$test_csv"
        rm -rf "$test_dir"
        return 1
    fi

    # Create tar.gz file
    echo_info "Creating tar.gz archive..."
    if ! (cd "$test_dir" && tar -czf "$tar_filename" "$csv_filename"); then
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
    local response=$(curl -s -w "%{http_code}" \
        -F "file=@${test_dir}/${tar_filename};type=application/vnd.redhat.hccm.filename+tgz" \
        -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K" \
        -H "x-rh-request-id: test-request-$(date +%s)" \
        http://localhost:${INGRESS_PORT}/api/ingress/v1/upload)

    local http_code="${response: -3}"
    local response_body="${response%???}"

    # Cleanup
    rm -f "$test_csv"
    rm -rf "$test_dir"

    if [ "$http_code" != "202" ]; then
        echo_error "Upload failed! HTTP $http_code"
        echo_error "Response: $response_body"
        return 1
    fi

    echo_success "Upload successful! HTTP $http_code"
    echo_info "Response: $response_body"

    return 0
}

# Function to simulate Koku processing
simulate_koku_processing() {
    echo_info "=== STEP 2: Simulate Koku Processing ===="

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

    # Create test CSV content
    local csv_content='report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
2024-01-01,2024-01-01,2024-01-01 00:00:00 -0000 UTC,2024-01-01 00:15:00 -0000 UTC,test-container,test-pod-123,test-deployment,Deployment,test-workload,deployment,test-namespace,quay.io/test/image:latest,worker-node-1,resource-123,100,100,200,200,50,10,90,50,0,0,0,512,512,1024,1024,256,128,384,256,200,100,300,200'

    # Copy CSV data to ros-data bucket via MinIO pod
    echo_info "Copying CSV to ros-data bucket..."

    local minio_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=minio" -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$minio_pod" ]; then
        echo_error "MinIO pod not found"
        return 1
    fi

    # Create CSV file in MinIO pod and copy to bucket
    # Use stdin redirection to avoid websocket stream issues with large content
    echo "$csv_content" | kubectl exec -i -n "$NAMESPACE" "$minio_pod" -- sh -c "
        cat > /tmp/$csv_filename
        /usr/bin/mc alias set myminio http://localhost:9000 minioaccesskey miniosecretkey 2>/dev/null
        /usr/bin/mc cp /tmp/$csv_filename myminio/ros-data/$csv_filename
        rm /tmp/$csv_filename
    "

    if [ $? -ne 0 ]; then
        echo_error "Failed to copy CSV to ros-data bucket"
        return 1
    fi

    echo_success "CSV file copied to ros-data bucket"

    # Verify file accessibility from processor pod
    local processor_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rosocp-processor" -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$processor_pod" ]; then
        echo_info "Verifying file accessibility from processor pod..."

        local file_url="http://${HELM_RELEASE_NAME}-minio:9000/ros-data/$csv_filename"
        local access_test=$(kubectl exec -n "$NAMESPACE" "$processor_pod" -- curl -s -I "$file_url" | head -1)

        if [[ "$access_test" =~ "200 OK" ]]; then
            echo_success "File is accessible via HTTP"
        else
            echo_error "File is not accessible via HTTP: $access_test"
        fi
    fi

    echo_info "=== STEP 3: Publish Kafka Event ===="

    # Create Kafka message with container network URL (compact JSON)
    local kafka_message="{\"request_id\":\"test-request-$(date +%s)\",\"b64_identity\":\"eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K\",\"metadata\":{\"account\":\"12345\",\"org_id\":\"12345\",\"source_id\":\"test-source-id\",\"cluster_uuid\":\"1b77b73f-1d3e-43c6-9f55-bcd9fb6d1a0c\",\"cluster_alias\":\"test-cluster\"},\"files\":[\"http://${HELM_RELEASE_NAME}-minio:9000/ros-data/$csv_filename\"]}"

    echo_info "Publishing Kafka message to hccm.ros.events topic"
    echo_info "Message content: $kafka_message"

    # Get Kafka pod
    local kafka_pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=kafka" -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$kafka_pod" ]; then
        echo_error "Kafka pod not found"
        return 1
    fi

    # Publish message to Kafka
    echo "$kafka_message" | kubectl exec -i -n "$NAMESPACE" "$kafka_pod" -- \
        kafka-console-producer --broker-list localhost:29092 --topic hccm.ros.events

    if [ $? -eq 0 ]; then
        echo_success "Kafka message published successfully"
        echo_info "File UUID: $file_uuid"
        echo_info "CSV file: $csv_filename"
        echo_info "Accessible at: http://localhost:30099/browser/ros-data/$csv_filename"
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

# Function to run health checks
run_health_checks() {
    echo_info "=== Health Checks ===="

    local failed_checks=0

    # Check ingress API
    if curl -f -s http://localhost:${INGRESS_PORT}/api/ingress/v1/version >/dev/null; then
        echo_success "Ingress API is accessible"
    else
        echo_error "Ingress API is not accessible"
        failed_checks=$((failed_checks + 1))
    fi

    # Check ROS-OCP API
    if curl -f -s http://localhost:${API_PORT}/status >/dev/null; then
        echo_success "ROS-OCP API is accessible"
    else
        echo_error "ROS-OCP API is not accessible"
        failed_checks=$((failed_checks + 1))
    fi

    # Check Kruize API
    if curl -f -s http://localhost:${KRUIZE_PORT}/listPerformanceProfiles >/dev/null; then
        echo_success "Kruize API is accessible"
    else
        echo_error "Kruize API is not accessible"
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

    echo_info "Configuration:"
    echo_info "  Namespace: $NAMESPACE"
    echo_info "  Helm Release: $HELM_RELEASE_NAME"
    echo_info "  Ingress Port: $INGRESS_PORT"
    echo_info "  API Port: $API_PORT"
    echo_info "  Kruize Port: $KRUIZE_PORT"
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

    echo ""
    run_health_checks

    echo ""
    echo_success "Data flow test completed!"
    echo_info "Use '$0 logs <service>' to view specific service logs"
    echo_info "Available services: ingress, rosocp-processor, rosocp-api, kruize, minio, db-ros"
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
    "help"|"-h"|"--help")
        echo "Usage: $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  (none)      - Run complete data flow test"
        echo "  logs [svc]  - Show logs for service (or list services if no service specified)"
        echo "  health      - Run health checks only"
        echo "  help        - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  NAMESPACE         - Kubernetes namespace (default: ros-ocp)"
        echo "  HELM_RELEASE_NAME - Helm release name (default: ros-ocp)"
        echo "  INGRESS_PORT      - Ingress service port (default: 30080)"
        echo "  API_PORT          - API service port (default: 30081)"
        echo "  KRUIZE_PORT       - Kruize service port (default: 30090)"
        exit 0
        ;;
esac

# Run main function
main "$@"