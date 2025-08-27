# ROS-OCP Backend Testing Scripts

This directory contains scripts and configuration for testing the complete ROS-OCP backend data flow using podman-compose.

## Quick Start

```bash
# Navigate to the scripts directory
cd ros-ocp-backend/scripts/

# Set environment variables (optional, defaults will be used)
export INGRESS_PORT=3000
export MINIO_ACCESS_KEY=minioaccesskey
export MINIO_SECRET_KEY=miniosecretkey

# Run the complete data flow test
./test-ros-ocp-dataflow.sh
```

## Files Overview

- `docker-compose.yml` - Base compose file with all service definitions
- `docker-compose.override.yml` - Override file with insights-onprem images and MinIO services
- `test-ros-ocp-dataflow.sh` - Comprehensive test script for the entire data flow
- `cdappconfig.json` - Kruize database configuration
- `samples/` - Sample data files for testing
- `minio-data/` - MinIO persistent storage directory

## Test Script Features

The `test-ros-ocp-dataflow.sh` script provides:

### 🚀 Service Management
- Starts all services using podman-compose with proper dependencies
- Waits for services to be healthy before proceeding
- Shows service status and logs
- Provides cleanup functionality

### 📊 Complete Data Flow Testing
1. **Upload Phase**: Tests file upload via Ingress API
2. **Storage Phase**: Verifies files are stored in MinIO bucket
3. **Messaging Phase**: Checks Kafka events are produced
4. **Processing Phase**: Confirms data is processed and stored in database

### 🔍 Verification Steps
- **MinIO Bucket**: Checks if uploaded files are stored
- **Kafka Topics**: Monitors `hccm.ros.events` and `rosocp.kruize.recommendations` topics
- **Database**: Verifies workload data is inserted into PostgreSQL
- **Service Health**: Confirms all services are running and responsive

## Usage Examples

### Run Complete Test
```bash
./test-ros-ocp-dataflow.sh
```

### Check Service Status
```bash
./test-ros-ocp-dataflow.sh status
```

### View Logs
```bash
# View all logs
./test-ros-ocp-dataflow.sh logs

# View specific service logs
./test-ros-ocp-dataflow.sh logs rosocp-processor
./test-ros-ocp-dataflow.sh logs ingress
./test-ros-ocp-dataflow.sh logs kruize-autotune
```

### Clean Up
```bash
./test-ros-ocp-dataflow.sh cleanup
```

## Services Started

The script starts the following services:

### Core Infrastructure
- **PostgreSQL** (3 instances): ROS, Kruize, and Sources databases
- **Kafka + Zookeeper**: Message streaming
- **Redis**: Caching
- **MinIO**: Object storage with bucket setup

### Application Services
- **Ingress**: File upload API (`localhost:3000`)
- **ROS-OCP API**: Main API service (`localhost:8001`)
- **ROS-OCP Processor**: Processes uploaded data
- **ROS-OCP Recommendation Poller**: Polls Kruize for recommendations
- **ROS-OCP Housekeeper**: Cleanup and maintenance
- **Kruize Autotune**: Recommendation engine (`localhost:8080`)
- **Sources API**: Sources management (`localhost:8002`)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INGRESS_PORT` | `3000` | Port for Ingress API |
| `MINIO_ACCESS_KEY` | `minioaccesskey` | MinIO access key |
| `MINIO_SECRET_KEY` | `miniosecretkey` | MinIO secret key |

## Test Data

The script uses test data from:
- `docs/examples/cost-mgmt report/cost-mgmt.tar.gz` - Primary test file
- `samples/cost-mgmt.tar.gz` - Alternative test file

## Data Flow Verification

The script verifies the complete data flow:

```
Upload File → Ingress API → MinIO Storage → Kafka Event →
ROS Processor → Kruize API → Database Storage
```

### Step-by-Step Verification:

1. **File Upload**: POST to `/api/ingress/v1/upload` with tar.gz file
2. **MinIO Check**: Verifies file is stored in `insights-upload-perma` bucket
3. **Kafka Check**: Confirms events in `hccm.ros.events` topic
4. **Processing Check**: Monitors `rosocp.kruize.recommendations` topic
5. **Database Check**: Verifies workload records in PostgreSQL

## Troubleshooting

### Common Issues

**Services not starting**:
```bash
# Check service logs
./test-ros-ocp-dataflow.sh logs [service-name]

# Check overall status
podman-compose ps
```

**Upload failures**:
- Ensure Ingress service is running: `curl http://localhost:3000/api/ingress/v1/version`
- Check MinIO is accessible: `curl http://localhost:9000/minio/health/live`
- Verify Kafka is ready: `podman exec scripts_kafka_1 kafka-topics --list --bootstrap-server localhost:29092`

**No data in database**:
- Check processor logs: `./test-ros-ocp-dataflow.sh logs rosocp-processor`
- Verify Kruize is responding: `curl http://localhost:8080/listPerformanceProfiles`
- Check database connection: `podman exec scripts_db-ros_1 pg_isready -U postgres`

### Manual Testing

**Upload test file manually**:
```bash
curl -F "file=@docs/examples/cost-mgmt report/cost-mgmt.tar.gz" \
     -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMTIzNDUifX19" \
     http://localhost:3000/api/ingress/v1/upload
```

**Check MinIO bucket**:
```bash
podman exec scripts_minio_1 /usr/bin/mc ls myminio/insights-upload-perma/
```

**Monitor Kafka topics**:
```bash
podman exec scripts_kafka_1 kafka-console-consumer \
    --bootstrap-server localhost:29092 \
    --topic hccm.ros.events \
    --from-beginning
```

**Query database**:
```bash
podman exec scripts_db-ros_1 psql -U postgres -d postgres -c "SELECT COUNT(*) FROM workloads;"
```

## Access Points

After running the script, these endpoints are available:

- **Ingress API**: http://localhost:3000/api/ingress/v1/version
- **ROS-OCP API**: http://localhost:8001/status
- **Kruize API**: http://localhost:8080/listPerformanceProfiles
- **MinIO Console**: http://localhost:9990 (admin UI)
- **Sources API**: http://localhost:8002/api/sources/v1.0/source_types

## Notes

- The script uses podman-compose as specified in the project's CLAUDE.md guidelines
- All services are configured to use insights-onprem images where available
- MinIO bucket is automatically created and configured for public access
- The script includes comprehensive error handling and colored output for better readability
- Services remain running after the test completes for further manual testing