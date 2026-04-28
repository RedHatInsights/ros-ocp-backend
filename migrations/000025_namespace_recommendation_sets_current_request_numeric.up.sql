-- Alter namespace current request columns to fixed-precision numeric.
-- cpu_request_current stores cores; memory_request_current stores bytes.
ALTER TABLE namespace_recommendation_sets
    ALTER COLUMN cpu_request_current TYPE NUMERIC(10, 4),
    ALTER COLUMN memory_request_current TYPE NUMERIC(20, 4);

