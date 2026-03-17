-- Remove legacy aggregate variation columns; use per-term variation columns instead.
-- Keep cpu_request_current and memory_request_current.
ALTER TABLE namespace_recommendation_sets
    DROP COLUMN IF EXISTS cpu_variation,
    DROP COLUMN IF EXISTS memory_variation;

ALTER TABLE historical_namespace_recommendation_sets
    DROP COLUMN IF EXISTS cpu_variation,
    DROP COLUMN IF EXISTS memory_variation,
    DROP COLUMN IF EXISTS cpu_request_current,
    DROP COLUMN IF EXISTS memory_request_current;
