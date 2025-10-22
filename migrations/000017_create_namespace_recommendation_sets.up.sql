CREATE TABLE IF NOT EXISTS namespace_recommendation_sets(
   id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
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

ALTER TABLE namespace_recommendation_sets
ADD CONSTRAINT fk_namespace_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE namespace_recommendation_sets
ADD CONSTRAINT UQ_Namespace_Recommendation UNIQUE (workload_id);

CREATE INDEX IF NOT EXISTS idx_namespace_recommendation_sets_workload_id ON namespace_recommendation_sets (workload_id);
CREATE INDEX IF NOT EXISTS idx_namespace_recommendation_sets_monitoring_end_time ON namespace_recommendation_sets (monitoring_end_time);
