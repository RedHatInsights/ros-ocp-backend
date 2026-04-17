-- Roll back 000024: drop current request columns from recommendation_sets.
ALTER TABLE recommendation_sets
    DROP COLUMN IF EXISTS cpu_request_current,
    DROP COLUMN IF EXISTS memory_request_current;
