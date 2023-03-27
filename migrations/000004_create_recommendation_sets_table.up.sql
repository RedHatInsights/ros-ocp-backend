CREATE TABLE IF NOT EXISTS rosocp.recommendation_sets(
   id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
   workload_id BIGINT,
   container_name TEXT NOT NULL,
   monitoring_start_time TIMESTAMP WITH TIME ZONE NOT NULL,
   monitoring_end_time TIMESTAMP WITH TIME ZONE NOT NULL,
   recommendations jsonb NOT NULL,
   created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

ALTER TABLE rosocp.recommendation_sets
ADD CONSTRAINT fk_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES rosocp.workloads (id)
ON DELETE CASCADE;
