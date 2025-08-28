#!/bin/bash

# ROS-OCP Backend Data Flow Test Script
# This script starts all services using podman-compose and tests the complete data flow
# from upload through Kafka to database storage

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INGRESS_PORT=${INGRESS_PORT:-3000}
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-minioaccesskey}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-miniosecretkey}

# Set default values if not already set
if [ -z "$MINIO_ACCESS_KEY" ]; then
    MINIO_ACCESS_KEY="minioaccesskey"
fi
if [ -z "$MINIO_SECRET_KEY" ]; then
    MINIO_SECRET_KEY="miniosecretkey"
fi

echo_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Export environment variables for docker-compose
export INGRESS_PORT
export MINIO_ACCESS_KEY
export MINIO_SECRET_KEY
echo_info "Exported environment variables for docker-compose"

echo_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

echo_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to wait for service to be ready
wait_for_service() {
    local service_name="$1"
    local check_command="$2"
    local timeout="$3"
    local counter=0

    echo_info "Waiting for $service_name to be ready..."

    while [ $counter -lt $timeout ]; do
        if eval "$check_command" >/dev/null 2>&1; then
            echo_success "$service_name is ready!"
            return 0
        fi

        sleep 5
        counter=$((counter + 5))
        echo_info "Waiting for $service_name... (${counter}s/${timeout}s)"
    done

    echo_error "$service_name failed to start within ${timeout}s"
    return 1
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to ensure uuidgen is available
ensure_uuidgen() {
    if ! command_exists uuidgen; then
        echo_warning "uuidgen not found, attempting to install..."
        if command_exists brew; then
            brew install util-linux 2>/dev/null || true
        elif command_exists apt-get; then
            sudo apt-get update && sudo apt-get install -y uuid-runtime 2>/dev/null || true
        elif command_exists dnf; then
            sudo dnf install -y util-linux 2>/dev/null || true
        fi

        if ! command_exists uuidgen; then
            echo_error "Could not install uuidgen. Please install it manually."
            return 1
        fi
    fi
    return 0
}

# Function to stop all services
cleanup() {
    echo_info "Cleaning up services..."
    cd "$SCRIPT_DIR"
    podman-compose down -v || true
    echo_success "Cleanup completed"
}

# Function to upload test data and simulate complete data flow
upload_test_data() {
    local upload_file="$1"
    local expected_topic="$2"

    echo_info "=== STEP 1: Upload to Ingress Service ==="
    echo_info "Uploading test data: $upload_file"

    # Create a temporary file with proper headers
    local temp_file=$(mktemp)

    # Upload the file using curl with proper content type
    local response=$(curl -s -w "%{http_code}" \
        -F "upload=@${upload_file};type=application/vnd.redhat.hccm.tar+tgz" \
        -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0=" \
        -H "x-rh-request-id: test-request-$(date +%s)" \
        http://localhost:${ACTUAL_INGRESS_PORT:-$INGRESS_PORT}/api/ingress/v1/upload)

    local http_code="${response: -3}"
    local response_body="${response%???}"

    rm -f "$temp_file"

    if [ "$http_code" != "202" ]; then
        echo_error "Upload failed! HTTP $http_code"
        echo_error "Response: $response_body"
        return 1
    fi

    echo_success "Upload successful! HTTP $http_code"
    echo_info "Response: $response_body"

    # Wait for file to appear in insights-upload-perma bucket
    echo_info "Waiting for file to appear in insights-upload-perma bucket..."
    sleep 10

    echo_info "=== STEP 2: Simulate Koku Service Role ==="

    # Ensure uuidgen is available
    if ! ensure_uuidgen; then
        return 1
    fi

    # Generate unique file UUID for this test
    local file_uuid
    file_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    local csv_filename="${file_uuid}_openshift_usage_report.0.csv"

    # Use ros-ocp-usage.csv (correct format) instead of uploaded file
    local source_csv="$SCRIPT_DIR/samples/ros-ocp-usage.csv"
    if [ ! -f "$source_csv" ]; then
        echo_error "Required CSV file not found: $source_csv"
        return 1
    fi

    echo_info "Copying $source_csv to ros-data bucket as $csv_filename"

    # Copy CSV data to ros-data bucket (simulating koku service)
    # Use the main minio container and set up alias if needed
    podman exec minio_1 /usr/bin/mc alias set myminio http://localhost:9000 minioaccesskey miniosecretkey >/dev/null 2>&1 || true
    
    # Copy the file to the container and then to MinIO
    podman cp "$source_csv" minio_1:/tmp/"$csv_filename"
    podman exec minio_1 /usr/bin/mc cp /tmp/"$csv_filename" myminio/ros-data/"$csv_filename"
    podman exec minio_1 rm -f /tmp/"$csv_filename"

    if [ $? -ne 0 ]; then
        echo_error "Failed to copy CSV to ros-data bucket"
        return 1
    fi

    echo_success "CSV file copied to ros-data bucket"

    echo_info "=== STEP 3: Verify File Accessibility ==="

    # Verify file is accessible via HTTP (using container network URL)
    local file_url="http://minio:9000/ros-data/$csv_filename"
    echo_info "Verifying file accessibility at: $file_url"

    # Test accessibility from within container network
    local access_test=$(podman exec rosocp-processor_1 curl -s -I "$file_url" | head -1)
    if [[ "$access_test" =~ "200 OK" ]]; then
        echo_success "File is accessible via HTTP"
    else
        echo_error "File is not accessible via HTTP: $access_test"
        return 1
    fi

    echo_info "=== STEP 4: Publish Kafka Event ==="

    # Create Kafka message with container network URL (compact JSON for single-line publishing)
    local kafka_message="{\"request_id\":\"test-request-$(date +%s)\",\"b64_identity\":\"eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0=\",\"metadata\":{\"account\":\"12345\",\"org_id\":\"12345\",\"source_id\":\"test-source-id\",\"cluster_uuid\":\"1b77b73f-1d3e-43c6-9f55-bcd9fb6d1a0c\",\"cluster_alias\":\"test-cluster\"},\"files\":[\"$file_url\"]}"

    echo_info "Publishing Kafka message to $expected_topic"
    echo_info "Message content: $kafka_message"

    # Publish message to Kafka
    echo "$kafka_message" | podman exec -i kafka_1 kafka-console-producer \
        --broker-list localhost:29092 \
        --topic "$expected_topic"

    if [ $? -eq 0 ]; then
        echo_success "Kafka message published successfully"
        echo_info "=== Complete data flow simulation finished ==="
        echo_info "File UUID: $file_uuid"
        echo_info "CSV file: $csv_filename"
        echo_info "Accessible at: $file_url"
        return 0
    else
        echo_error "Failed to publish Kafka message"
        return 1
    fi
}

# Function to check MinIO bucket contents
check_minio_bucket() {
    echo_info "Checking MinIO bucket contents..."

    # Use podman exec to check MinIO bucket
    local bucket_contents=$(podman exec minio_1 /usr/bin/mc ls myminio/insights-upload-perma/ 2>/dev/null || echo "")

    if [ -n "$bucket_contents" ]; then
        echo_success "Files found in MinIO bucket:"
        echo "$bucket_contents"
        return 0
    else
        echo_warning "No files found in MinIO bucket yet"
        return 1
    fi
}

# Function to check Kafka topics and messages with retry logic
check_kafka_events_with_retry() {
    local topic="$1"
    local max_retries="${2:-3}"
    local retry_delay="${3:-30}"
    
    echo_info "Checking Kafka topic: $topic (with retry)"
    
    # List topics first
    local topics=$(podman exec kafka_1 kafka-topics --list --bootstrap-server localhost:29092 2>/dev/null || echo "")
    
    if ! echo "$topics" | grep -q "$topic"; then
        echo_error "Topic $topic does not exist"
        echo_info "Available topics: $topics"
        return 1
    fi
    
    echo_success "Topic $topic exists"
    
    # Try to consume messages with retries
    for attempt in $(seq 1 $max_retries); do
        echo_info "Attempt $attempt/$max_retries: Checking for messages in topic $topic..."
        
        local messages=$(podman exec kafka_1 kafka-console-consumer \
            --bootstrap-server localhost:29092 \
            --topic "$topic" \
            --from-beginning \
            --max-messages 5 \
            --timeout-ms 10000 2>/dev/null || echo "")
        
        if [ -n "$messages" ]; then
            echo_success "Found messages in topic $topic:"
            echo "$messages" | head -5
            return 0
        else
            if [ $attempt -lt $max_retries ]; then
                echo_warning "No messages found in topic $topic yet (attempt $attempt/$max_retries)"
                echo_info "Waiting ${retry_delay}s before retry..."
                sleep $retry_delay
            else
                echo_warning "No messages found in topic $topic after $max_retries attempts"
                return 1
            fi
        fi
    done
}

# Function to check Kafka topics and messages
check_kafka_events() {
    local topic="$1"

    echo_info "Checking Kafka topic: $topic"

    # List topics first
    local topics=$(podman exec kafka_1 kafka-topics --list --bootstrap-server localhost:29092 2>/dev/null || echo "")

    if echo "$topics" | grep -q "$topic"; then
        echo_success "Topic $topic exists"

        # Try to consume recent messages
        echo_info "Checking for recent messages in topic $topic..."
        local messages=$(podman exec kafka_1 kafka-console-consumer \
            --bootstrap-server localhost:29092 \
            --topic "$topic" \
            --from-beginning \
            --max-messages 5 \
            --timeout-ms 10000 2>/dev/null || echo "")

        if [ -n "$messages" ]; then
            echo_success "Found messages in topic $topic:"
            echo "$messages" | head -5
            return 0
        else
            echo_warning "No messages found in topic $topic yet"
            return 1
        fi
    else
        echo_error "Topic $topic does not exist"
        echo_info "Available topics: $topics"
        return 1
    fi
}

# Function to verify data processing (enhanced like k8s test)
verify_processing() {
    echo_info "=== VERIFICATION: Data Processing ==="
    
    echo_info "Checking processor logs for recent activity..."
    local processor_logs=$(podman logs rosocp-processor_1 --tail=15 | grep -E "(Message received|Recommendation request sent|DB initialization complete)" | tail -5 || echo "")
    
    if [ -n "$processor_logs" ]; then
        echo_success "Processor is active - recent processing logs:"
        echo "$processor_logs"
    else
        echo_warning "No recent processor activity found"
    fi
    
    echo_info "Checking database for workload records..."
    local row_count=$(podman exec db-ros_1 psql -U postgres -d postgres -t -c \
        "SELECT COUNT(*) FROM workloads;" 2>/dev/null | tr -d ' ' || echo "0")
    
    # Ensure we have a valid number
    if [ -z "$row_count" ] || ! [[ "$row_count" =~ ^[0-9]+$ ]]; then
        row_count="0"
    fi
    
    if [ "$row_count" -gt 0 ]; then
        echo_success "Found $row_count workload records in database"
        
        echo_info "Sample workload data:"
        podman exec db-ros_1 psql -U postgres -d postgres -c \
            "SELECT w.workload_name, w.workload_type, w.namespace, c.cluster_uuid FROM workloads w JOIN clusters c ON w.cluster_id = c.id LIMIT 3;" 2>/dev/null || true
        
        # Check for kruize experiments
        echo_info "Checking Kruize experiments in database..."
        local kruize_experiments=$(podman exec db-kruize_1 psql -U postgres -d postgres -t -c \
            "SELECT COUNT(*) FROM kruize_experiments;" 2>/dev/null | tr -d ' ' || echo "0")
        
        # Ensure we have a valid number
        if [ -z "$kruize_experiments" ] || ! [[ "$kruize_experiments" =~ ^[0-9]+$ ]]; then
            kruize_experiments="0"
        fi
        
        if [ "$kruize_experiments" -gt 0 ]; then
            echo_success "Found $kruize_experiments Kruize experiments"
            echo_info "Sample Kruize experiments:"
            podman exec db-kruize_1 psql -U postgres -d postgres -c \
                "SELECT experiment_name, status FROM kruize_experiments LIMIT 3;" 2>/dev/null || true
        else
            echo_warning "No Kruize experiments found yet"
        fi
        
        return 0
    else
        echo_warning "No workload data found in database"
        return 1
    fi
}

# Function to check database for uploaded data (wrapper for backward compatibility)
check_database() {
    verify_processing
}

# Function to show service logs
show_service_logs() {
    local service="$1"
    echo_info "Showing logs for $service:"
    podman-compose logs --tail=20 "$service" || true
    echo ""
}

# Main execution
main() {
    echo_info "Starting ROS-OCP Backend Data Flow Test"
    echo_info "======================================="

    # Check prerequisites
    if ! command_exists podman-compose; then
        echo_error "podman-compose is not installed. Please install it first."
        exit 1
    fi

    if ! command_exists curl; then
        echo_error "curl is not installed. Please install it first."
        exit 1
    fi

    cd "$SCRIPT_DIR"

    # Setup cleanup trap
   # trap cleanup EXIT

    echo_info "Configuration:"
    echo_info "  INGRESS_PORT: $INGRESS_PORT"
    echo_info "  MINIO_ACCESS_KEY: $MINIO_ACCESS_KEY"
    echo_info "  MINIO_SECRET_KEY: $MINIO_SECRET_KEY"
    echo ""

    # Check if services are already running
    if podman exec db-ros_1 pg_isready -U postgres >/dev/null 2>&1; then
        echo_info "Services are already running, skipping startup..."
    else
        # Start services
        echo_info "Starting all services with podman-compose..."
        podman-compose up -d
    fi

    echo ""
    echo_info "Waiting for services to start..."

    # Get the actual ingress port from the running container
    ACTUAL_INGRESS_PORT=$(podman port ingress_1 3000 2>/dev/null | cut -d: -f2)
    if [ -z "$ACTUAL_INGRESS_PORT" ]; then
        ACTUAL_INGRESS_PORT=$INGRESS_PORT
    fi
    echo_info "Using ingress port: $ACTUAL_INGRESS_PORT"

    # Wait for core infrastructure services
    wait_for_service "PostgreSQL (ROS)" "podman exec db-ros_1 pg_isready -U postgres" 90
    wait_for_service "PostgreSQL (Kruize)" "podman exec db-kruize_1 pg_isready -U postgres" 90
    wait_for_service "PostgreSQL (Sources)" "podman exec db-sources_1 pg_isready -U postgres" 90
    wait_for_service "Kafka" "podman exec kafka_1 kafka-broker-api-versions --bootstrap-server localhost:29092" 90
    wait_for_service "MinIO" "curl -f http://localhost:9000/minio/health/live" 60
    wait_for_service "Redis" "podman exec redis_1 redis-cli ping" 60

    # Wait for application services
    wait_for_service "Ingress" "curl -f http://localhost:${ACTUAL_INGRESS_PORT}/api/ingress/v1/version" 120
    wait_for_service "Kruize" "curl -f http://localhost:8080/listPerformanceProfiles" 180
    wait_for_service "Sources API" "curl -f http://localhost:8002/api/sources/v1.0/source_types" 120
    wait_for_service "ROS-OCP API" "curl -f http://localhost:8001/status" 180
    wait_for_service "ROS-OCP Processor" "podman logs rosocp-processor_1 2>/dev/null | grep -q 'Starting processor'" 180
    wait_for_service "ROS-OCP Recommendation Poller" "podman logs rosocp-recommendation-poller_1 2>/dev/null | grep -q 'Starting recommendation-poller'" 180

    echo ""
    echo_success "All services are ready!"

    # Show service status
    echo_info "Service status:"
    podman-compose ps
    echo ""

    # Test 1: Upload cost management data
    echo_info "=== TEST 1: Upload Cost Management Data ==="
    local test_file="$SCRIPT_DIR/samples/cost-mgmt.tar.gz"

    if [ -f "$test_file" ]; then
        if upload_test_data "$test_file" "hccm.ros.events"; then
            echo_success "Upload test passed!"

            # Wait a bit for processing
            echo_info "Waiting for data processing..."
            sleep 30

            # Check MinIO bucket
            check_minio_bucket

            # Check Kafka events
            check_kafka_events "hccm.ros.events"
            
            # Wait for processing pipeline to complete before checking recommendations
            echo_info "Waiting for recommendation processing pipeline (processor → kruize → recommendations)..."
            sleep 30
            
            # Use retry logic for recommendations topic (3 attempts, 30s between retries)
            check_kafka_events_with_retry "rosocp.kruize.recommendations" 3 30

            # Check database
            sleep 30  # Give more time for database processing
            check_database

        else
            echo_error "Upload test failed!"
            show_service_logs "ingress"
        fi
    else
        echo_warning "Test file not found: $test_file"
        echo_info "Available files in samples:"
        ls -la "$SCRIPT_DIR/samples/" || true
    fi

    echo ""
    echo_info "=== TEST 2: Alternative Upload with Sample Data ==="

    # Try with sample data if available
    local sample_file="$SCRIPT_DIR/samples/ros-ocp-usage.csv"
    if [ -f "$sample_file" ]; then
        echo_info "Testing with sample file: $sample_file"
        upload_test_data "$sample_file" "hccm.ros.events"
    else
        echo_warning "No sample file found for additional testing"
    fi

    echo ""
    echo_info "=== FINAL STATUS ==="

    # Show final service status
    echo_info "Final service status:"
    podman-compose ps

    echo ""
    echo_info "Key service logs (last 10 lines each):"
    show_service_logs "rosocp-processor"
    show_service_logs "rosocp-api"
    show_service_logs "ingress"
    show_service_logs "kruize-autotune"

    echo ""
    echo_success "ROS-OCP Backend Data Flow Test completed!"
    echo_info "Services are still running. Use 'podman-compose down' to stop them."
    echo_info "Access points:"
    echo_info "  - Ingress API: http://localhost:${ACTUAL_INGRESS_PORT}/api/ingress/v1/version"
    echo_info "  - ROS-OCP API: http://localhost:8001/status"
    echo_info "  - Kruize API: http://localhost:8080/listPerformanceProfiles"
    echo_info "  - MinIO Console: http://localhost:9990 (user: ${MINIO_ACCESS_KEY})"
    echo_info "  - Sources API: http://localhost:8002/api/sources/v1.0/source_types"
}

# Handle script arguments
case "${1:-}" in
    "cleanup"|"stop")
        cleanup
        exit 0
        ;;
    "status")
        cd "$SCRIPT_DIR"
        podman-compose ps
        exit 0
        ;;
    "logs")
        cd "$SCRIPT_DIR"
        if [ -n "${2:-}" ]; then
            podman-compose logs -f "$2"
        else
            podman-compose logs
        fi
        exit 0
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  (none)    - Run the complete data flow test"
        echo "  cleanup   - Stop and clean up all services"
        echo "  status    - Show service status"
        echo "  logs [service] - Show logs for all services or specific service"
        echo "  help      - Show this help message"
        exit 0
        ;;
esac

# Run main function
main "$@"