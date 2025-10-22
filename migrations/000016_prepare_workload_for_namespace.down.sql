-- Workloads

ALTER TABLE workloads ALTER COLUMN containers SET NOT NULL;
ALTER TABLE workloads ALTER COLUMN workload_type SET NOT NULL;
ALTER TABLE workloads ALTER COLUMN workload_name SET NOT NULL;
ALTER TABLE workloads DROP COLUMN namespace_type;

-- Workload Metrics

ALTER TABLE workload_metrics DROP COLUMN namespace_name;
ALTER TABLE workload_metrics DROP COLUMN metric_type;
DROP TYPE metrictype;
