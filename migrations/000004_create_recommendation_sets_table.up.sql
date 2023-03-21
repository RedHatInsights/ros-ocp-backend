CREATE TABLE IF NOT EXISTS recommendation_sets(
   id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
   workload_id BIGINT,
   monitoring_start_time TIMESTAMP WITH TIME ZONE NOT NULL,
   monitoring_end_time TIMESTAMP WITH TIME ZONE NOT NULL,
   recommendations jsonb NOT NULL,
   created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

ALTER TABLE recommendation_sets
ADD CONSTRAINT fk_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;
