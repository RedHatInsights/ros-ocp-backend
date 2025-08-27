# Docker Compose Deployment

Local development deployment using Docker Compose (or preferably Podman Compose).

## Quick Start

```bash
# Navigate to this directory
cd deployment/docker-compose/

# Start all services
podman-compose up -d

# Check service status
podman-compose ps

# Run end-to-end testing
./test-ros-ocp-dataflow.sh
```

## Files

- `docker-compose.yml` - Base service definitions
- `docker-compose.override.yml` - Local development overrides with insights-onprem images

## Services Started

The compose setup includes:

### Core Infrastructure
- **PostgreSQL** (3 instances): ROS, Kruize, and Sources databases
- **Kafka + Zookeeper**: Message streaming
- **Redis**: Caching
- **MinIO**: Object storage

### Application Services
- **Ingress**: File upload API (port 3000)
- **ROS-OCP API**: Main API service (port 8001)
- **ROS-OCP Processor**: Processes uploaded data
- **ROS-OCP Recommendation Poller**: Polls Kruize for recommendations
- **ROS-OCP Housekeeper**: Cleanup and maintenance
- **Kruize Autotune**: Recommendation engine (port 8080)
- **Sources API**: Sources management (port 8002)

## Access Points

After startup, these endpoints are available:

- **Ingress API**: http://localhost:3000/api/ingress/v1/version
- **ROS-OCP API**: http://localhost:8001/status
- **Kruize API**: http://localhost:8080/listPerformanceProfiles
- **MinIO Console**: http://localhost:9990 (admin UI)
- **Sources API**: http://localhost:8002/api/sources/v1.0/source_types

## Environment Variables

Set these before running compose (optional, defaults provided):

```bash
export INGRESS_PORT=3000
export MINIO_ACCESS_KEY=minioaccesskey
export MINIO_SECRET_KEY=miniosecretkey
```

## Testing

Use the comprehensive test script:

```bash
# From project's home directory
cd deployment/docker-compose/

./test-ros-ocp-dataflow.sh

# Or run specific test commands
./test-ros-ocp-dataflow.sh status
./test-ros-ocp-dataflow.sh logs rosocp-processor
./test-ros-ocp-dataflow.sh cleanup
```

## Troubleshooting

**Services not starting**:
```bash
podman-compose logs [service-name]
```

**Check service health**:
```bash
curl http://localhost:3000/api/ingress/v1/version
curl http://localhost:8001/status
curl http://localhost:8080/listPerformanceProfiles
```

**Manual cleanup**:
```bash
podman-compose down -v
```

## Note

This setup follows the project's CLAUDE.md guidelines and uses Podman instead of Docker for improved security and compatibility with OpenShift environments.