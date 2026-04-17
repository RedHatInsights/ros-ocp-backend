ALTER TABLE recommendation_sets
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
