-- Roll back 000022: drop NUMERIC(10,4) * _pct columns, then restore legacy FLOAT variation columns
-- (same names as before 000022: cpu_variation_* / memory_variation_* without _pct).
ALTER TABLE namespace_recommendation_sets
    DROP COLUMN IF EXISTS cpu_variation_short_cost_pct,
    DROP COLUMN IF EXISTS cpu_variation_short_performance_pct,
    DROP COLUMN IF EXISTS cpu_variation_medium_cost_pct,
    DROP COLUMN IF EXISTS cpu_variation_medium_performance_pct,
    DROP COLUMN IF EXISTS cpu_variation_long_cost_pct,
    DROP COLUMN IF EXISTS cpu_variation_long_performance_pct,
    DROP COLUMN IF EXISTS memory_variation_short_cost_pct,
    DROP COLUMN IF EXISTS memory_variation_short_performance_pct,
    DROP COLUMN IF EXISTS memory_variation_medium_cost_pct,
    DROP COLUMN IF EXISTS memory_variation_medium_performance_pct,
    DROP COLUMN IF EXISTS memory_variation_long_cost_pct,
    DROP COLUMN IF EXISTS memory_variation_long_performance_pct;

ALTER TABLE namespace_recommendation_sets
    ADD COLUMN cpu_variation_short_cost FLOAT,
    ADD COLUMN cpu_variation_short_performance FLOAT,
    ADD COLUMN cpu_variation_medium_cost FLOAT,
    ADD COLUMN cpu_variation_medium_performance FLOAT,
    ADD COLUMN cpu_variation_long_cost FLOAT,
    ADD COLUMN cpu_variation_long_performance FLOAT,
    ADD COLUMN memory_variation_short_cost FLOAT,
    ADD COLUMN memory_variation_short_performance FLOAT,
    ADD COLUMN memory_variation_medium_cost FLOAT,
    ADD COLUMN memory_variation_medium_performance FLOAT,
    ADD COLUMN memory_variation_long_cost FLOAT,
    ADD COLUMN memory_variation_long_performance FLOAT;
