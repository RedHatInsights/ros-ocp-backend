-- GET Recommendations optimization

CREATE INDEX IF NOT EXISTS idx_cluster_last_reported_at ON clusters (last_reported_at);
CREATE INDEX IF NOT EXISTS idx_workloads_cluster_id ON workloads (cluster_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_set_workload_id ON recommendation_sets (workload_id);
