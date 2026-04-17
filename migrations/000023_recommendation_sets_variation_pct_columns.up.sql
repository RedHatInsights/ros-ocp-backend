-- Add per-term, per-engine variation percent-of-request columns for container recommendation_sets sorting
-- (same names and types as namespace_recommendation_sets.*_pct from 000022).
-- Existing and new rows keep NULL until the poller fills values; list queries use ORDER BY ... DESC NULLS LAST
-- so NULLs sort last (see listoptions.SQLOrderByFragment).
ALTER TABLE recommendation_sets
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
