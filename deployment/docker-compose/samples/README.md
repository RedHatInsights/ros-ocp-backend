# Test Sample Data

Sample data files for testing the ROS-OCP backend data processing pipeline.

## Files

### `cost-mgmt.tar.gz`
Compressed archive containing sample cost management data in the format expected by the Ingress API.

**Contents:**
- Sample CSV files with OpenShift usage data
- Proper ROS-OCP data format with required columns
- Realistic workload metrics for testing

**Usage:**
```bash
# Used automatically by Docker Compose test script
./test-ros-ocp-dataflow.sh

# Used by Kubernetes test script (from testing/scripts directory)
../../../testing/scripts/test-k8s-dataflow.sh

# Manual upload testing (Docker Compose)
curl -F "upload=@cost-mgmt.tar.gz;type=application/vnd.redhat.hccm.tar+tgz" \
     -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0=" \
     http://localhost:3000/api/ingress/v1/upload

# Manual upload testing (Kubernetes)
curl -F "file=@cost-mgmt.tar.gz;type=application/vnd.redhat.hccm.filename+tgz" \
     -H "x-rh-identity: eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1IiwidHlwZSI6IlVzZXIiLCJpbnRlcm5hbCI6eyJvcmdfaWQiOiIxMjM0NSJ9fX0K" \
     http://localhost:30080/api/ingress/v1/upload
```

### `ros-ocp-usage.csv`
Sample CSV file with ROS-OCP usage data.

**Format:**
- OpenShift container usage metrics
- CPU and memory utilization data
- Resource requests and limits
- Workload identification (namespace, pod, container)

### `ros-ocp-usage-24Hrs.csv`
Extended sample dataset covering 24 hours of usage data.

**Purpose:**
- More comprehensive testing scenarios
- Time-series data analysis
- Multi-interval workload patterns

## Data Format

All sample files follow the ROS-OCP expected format:

```csv
report_period_start,report_period_end,interval_start,interval_end,container_name,pod,owner_name,owner_kind,workload,workload_type,namespace,image_name,node,resource_id,cpu_request_container_avg,cpu_request_container_sum,cpu_limit_container_avg,cpu_limit_container_sum,cpu_usage_container_avg,cpu_usage_container_min,cpu_usage_container_max,cpu_usage_container_sum,cpu_throttle_container_avg,cpu_throttle_container_max,cpu_throttle_container_sum,memory_request_container_avg,memory_request_container_sum,memory_limit_container_avg,memory_limit_container_sum,memory_usage_container_avg,memory_usage_container_min,memory_usage_container_max,memory_usage_container_sum,memory_rss_usage_container_avg,memory_rss_usage_container_min,memory_rss_usage_container_max,memory_rss_usage_container_sum
```

## Key Requirements

### Time Format
- Use timezone format: `-0000 UTC` (not `Z`)
- Ensure interval durations are under 30 minutes for Kruize compatibility
- Match report periods with interval periods for short-duration data

### Data Quality
- Include realistic CPU and memory metrics
- Provide proper workload identification
- Use valid Kubernetes resource names
- Include container image references

## Testing Integration

These sample files are used by:

1. **Upload Testing** - Files uploaded via Ingress API
2. **Processing Testing** - Data processed by ROS-OCP processor
3. **Kruize Integration** - Data sent to Kruize for optimization analysis
4. **Database Verification** - Workload data stored and queryable

## Creating New Sample Data

When creating new test data:

1. **Follow the exact CSV format** with all required columns
2. **Use proper time formatting** compatible with Go time parsing
3. **Keep interval durations short** (15-30 minutes) for Kruize validation
4. **Include realistic metrics** that represent actual workload patterns
5. **Test the data** with both deployment methods before committing

## Notes

- Sample data uses account number `12345` for consistency
- All workload names use `test-` prefix for identification
- Data is designed to trigger Kruize optimization analysis
- Files are safe for automated testing and won't affect production systems