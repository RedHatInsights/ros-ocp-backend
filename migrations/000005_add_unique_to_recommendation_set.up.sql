ALTER TABLE recommendation_sets
ADD CONSTRAINT UQ_Recommendation UNIQUE (workload_id, container_name, monitoring_start_time, monitoring_end_time);