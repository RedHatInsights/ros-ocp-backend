-- Replace variation columns: drop legacy raw columns from 000020, then add percent-of-request columns with *_pct names.
ALTER TABLE namespace_recommendation_sets
    DROP COLUMN IF EXISTS cpu_variation_short_cost,
    DROP COLUMN IF EXISTS cpu_variation_short_performance,
    DROP COLUMN IF EXISTS cpu_variation_medium_cost,
    DROP COLUMN IF EXISTS cpu_variation_medium_performance,
    DROP COLUMN IF EXISTS cpu_variation_long_cost,
    DROP COLUMN IF EXISTS cpu_variation_long_performance,
    DROP COLUMN IF EXISTS memory_variation_short_cost,
    DROP COLUMN IF EXISTS memory_variation_short_performance,
    DROP COLUMN IF EXISTS memory_variation_medium_cost,
    DROP COLUMN IF EXISTS memory_variation_medium_performance,
    DROP COLUMN IF EXISTS memory_variation_long_cost,
    DROP COLUMN IF EXISTS memory_variation_long_performance;

ALTER TABLE namespace_recommendation_sets
    ADD COLUMN cpu_variation_short_cost_pct NUMERIC(10, 4),
    ADD COLUMN cpu_variation_short_performance_pct NUMERIC(10, 4),
    ADD COLUMN cpu_variation_medium_cost_pct NUMERIC(10, 4),
    ADD COLUMN cpu_variation_medium_performance_pct NUMERIC(10, 4),
    ADD COLUMN cpu_variation_long_cost_pct NUMERIC(10, 4),
    ADD COLUMN cpu_variation_long_performance_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_short_cost_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_short_performance_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_medium_cost_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_medium_performance_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_long_cost_pct NUMERIC(10, 4),
    ADD COLUMN memory_variation_long_performance_pct NUMERIC(10, 4);
