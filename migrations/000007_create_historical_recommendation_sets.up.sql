CREATE TABLE IF NOT EXISTS historical_recommendation_sets AS TABLE recommendation_sets;

ALTER TABLE historical_recommendation_sets ADD PRIMARY KEY (id);
ALTER TABLE historical_recommendation_sets ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE historical_recommendation_sets ALTER COLUMN id SET NOT NULL;
ALTER TABLE historical_recommendation_sets ALTER COLUMN container_name SET NOT NULL;
ALTER TABLE historical_recommendation_sets ALTER COLUMN monitoring_start_time SET NOT NULL;
ALTER TABLE historical_recommendation_sets ALTER COLUMN monitoring_end_time SET NOT NULL;
ALTER TABLE historical_recommendation_sets ALTER COLUMN recommendations SET NOT NULL;
ALTER TABLE historical_recommendation_sets ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT fk_historical_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT UQ_historical_recommendation UNIQUE (workload_id, container_name, monitoring_end_time);