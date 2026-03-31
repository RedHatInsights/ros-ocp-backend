-- Restore legacy variation columns for rollback (nullable).
ALTER TABLE namespace_recommendation_sets
    ADD COLUMN IF NOT EXISTS cpu_variation FLOAT,
    ADD COLUMN IF NOT EXISTS memory_variation FLOAT;

ALTER TABLE historical_namespace_recommendation_sets
    ADD COLUMN IF NOT EXISTS cpu_variation FLOAT,
    ADD COLUMN IF NOT EXISTS memory_variation FLOAT,
    ADD COLUMN IF NOT EXISTS cpu_request_current FLOAT,
    ADD COLUMN IF NOT EXISTS memory_request_current FLOAT;
