CREATE TABLE IF NOT EXISTS workload_metrics(
   id BIGSERIAL PRIMARY KEY,
   workload_id BIGINT NOT NULL,
   container_name TEXT NOT NULL,
   interval_start TIMESTAMP WITH TIME ZONE NOT NULL,
   interval_end TIMESTAMP WITH TIME ZONE NOT NULL,
   usage_metrics jsonb NOT NULL
);

ALTER TABLE workload_metrics
ADD CONSTRAINT fk_workload_metrics_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE workload_metrics
ADD CONSTRAINT UQ_Workload_Metrics UNIQUE (workload_id, container_name, interval_start, interval_end);