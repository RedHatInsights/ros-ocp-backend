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
