-- Workloads

ALTER TABLE workloads ALTER COLUMN containers DROP NOT NULL;
ALTER TABLE workloads ALTER COLUMN workload_type DROP NOT NULL;
ALTER TABLE workloads ALTER COLUMN workload_name DROP NOT NULL;

-- Workload Metrics

ALTER TABLE workload_metrics ADD COLUMN namespace_name TEXT;

CREATE TYPE metrictype AS ENUM ('container', 'namespace');
ALTER TABLE workload_metrics ADD COLUMN metric_type metrictype DEFAULT 'container';
