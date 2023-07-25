ALTER TABLE recommendation_sets DROP CONSTRAINT IF EXISTS UQ_Recommendation;
ALTER TABLE recommendation_sets
ADD CONSTRAINT UQ_Recommendation UNIQUE (workload_id, container_name);