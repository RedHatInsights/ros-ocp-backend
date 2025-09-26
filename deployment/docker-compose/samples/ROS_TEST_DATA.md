# ROS-OCP Test Data

## ros-ocp-test-data.tar.gz

This archive contains properly formatted test data for the ROS-OCP data flow testing.

### Contents

- **manifest.json**: Metadata file describing the upload
- **CSV files**: Three CSV files with complete ROS-OCP column structure
  - `023d9b0e-7ca6-481d-b04f-ea606becd54e_ros_openshift_usage_report.0.csv`
  - `023d9b0e-7ca6-481d-b04f-ea606becd54e_ros_openshift_usage_report.1.csv` 
  - `023d9b0e-7ca6-481d-b04f-ea606becd54e_ros_openshift_usage_report.2.csv`

### Data Structure

Each CSV file contains all 37 required columns as defined in `CSVColumnMapping`:

**Metadata columns:**
- report_period_start, report_period_end
- interval_start, interval_end
- container_name, pod, owner_name, owner_kind
- workload, workload_type, namespace
- image_name, node, resource_id

**CPU metrics:**
- cpu_request_container_avg, cpu_request_container_sum
- cpu_limit_container_avg, cpu_limit_container_sum
- cpu_usage_container_avg, cpu_usage_container_min, cpu_usage_container_max, cpu_usage_container_sum
- cpu_throttle_container_avg, cpu_throttle_container_max, cpu_throttle_container_sum

**Memory metrics:**
- memory_request_container_avg, memory_request_container_sum
- memory_limit_container_avg, memory_limit_container_sum
- memory_usage_container_avg, memory_usage_container_min, memory_usage_container_max, memory_usage_container_sum
- memory_rss_usage_container_avg, memory_rss_usage_container_min, memory_rss_usage_container_max, memory_rss_usage_container_sum

### Test Data Content

The test data simulates various Kubernetes workloads:

1. **Web Application** (Deployment)
   - Namespace: default
   - Workload: web-app
   - Container: nginx:1.21

2. **API Service** (Deployment)
   - Namespace: backend
   - Workload: api-app
   - Container: golang:1.19

3. **Database** (StatefulSet)
   - Namespace: database
   - Workload: postgres-db
   - Container: postgres:13

4. **Cache Service** (Deployment)
   - Namespace: cache
   - Workload: redis
   - Container: redis:6.2

5. **Background Worker** (Deployment)
   - Namespace: processing
   - Workload: background-worker
   - Container: python:3.9

6. **Monitoring** (DaemonSet)
   - Namespace: monitoring
   - Workload: prometheus-monitoring
   - Container: prometheus:v2.40

7. **Logging** (DaemonSet)
   - Namespace: logging
   - Workload: fluentd-logging
   - Container: fluentd:v1.14

### Cluster Information

- **Cluster ID**: `023d9b0e-7ca6-481d-b04f-ea606becd54e`
- **Date Range**: 2023-09-10 13:00:00 to 2023-09-10 15:00:00
- **Account**: 1
- **Org ID**: 1

### Validation

This test data passes all validation checks in `aggregator.go`:

1. ✅ **Column validation**: All 37 required columns present
2. ✅ **Data validation**: All numeric values ≥ 0
3. ✅ **Workload validation**: Valid workload types (deployment, statefulset, daemonset)
4. ✅ **Required fields**: No empty owner_kind, owner_name, or workload_type

### Usage in Tests

The test script `test-ros-ocp-dataflow.sh` now uses this data by default, which should result in:

- ✅ Successful CSV validation
- ✅ Data processing without "Invalid records" errors
- ✅ Workload records inserted into database
- ✅ Kruize experiments created
- ✅ Full data pipeline execution

### Migration from cost-mgmt.tar.gz

The original `cost-mgmt.tar.gz` contained incomplete data with only 6 columns, causing validation failures. This new test data provides complete ROS-OCP formatted data for proper end-to-end testing.