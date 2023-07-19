ALTER TABLE historical_recommendation_sets RENAME TO recommendation_sets;

ALTER TABLE recommendation_sets
ADD CONSTRAINT fk_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE recommendation_sets DROP CONSTRAINT IF EXISTS fk_historical_recommendation_sets_workload;
