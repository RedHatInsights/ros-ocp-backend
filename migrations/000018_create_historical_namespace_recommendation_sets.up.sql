CREATE TABLE IF NOT EXISTS historical_namespace_recommendation_sets(
   id BIGSERIAL NOT NULL,
   org_id TEXT NOT NULL,
   workload_id BIGINT NOT NULL,
   namespace_name TEXT NOT NULL,
   cpu_request_current FLOAT,
   cpu_variation FLOAT,
   memory_request_current FLOAT,
   memory_variation FLOAT,
   monitoring_start_time TIMESTAMP WITH TIME ZONE NOT NULL,
   monitoring_end_time TIMESTAMP WITH TIME ZONE NOT NULL,
   recommendations jsonb NOT NULL,
   updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

ALTER TABLE historical_namespace_recommendation_sets
ADD CONSTRAINT fk_historical_namespace_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE historical_namespace_recommendation_sets
ADD CONSTRAINT UQ_historical_namespace_recommendation UNIQUE (org_id, workload_id, monitoring_end_time);
