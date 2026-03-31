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
