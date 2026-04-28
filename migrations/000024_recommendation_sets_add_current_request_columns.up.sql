-- Add current request columns to recommendation_sets for sorting (mirrors namespace_recommendation_sets).
-- Existing rows remain NULL; the poller fills values for new records.
-- List queries use ORDER BY ... DESC NULLS LAST (see listoptions.SQLOrderByFragment).
ALTER TABLE recommendation_sets
    ADD COLUMN cpu_request_current NUMERIC(10, 4),
    ADD COLUMN memory_request_current NUMERIC(20, 4);
