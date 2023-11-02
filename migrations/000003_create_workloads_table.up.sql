CREATE TYPE workloadtype AS ENUM ('daemonset', 'deployment', 'deploymentconfig', 'replicaset', 'replicationcontroller', 'statefulset');

CREATE TABLE IF NOT EXISTS workloads(
   id BIGSERIAL PRIMARY KEY,
   org_id TEXT NOT NULL,
   cluster_id BIGINT NOT NULL,
   experiment_name TEXT NOT NULL,
   namespace TEXT NOT NULL,
   workload_type workloadtype NOT NULL,
   workload_name TEXT NOT NULL,
   containers TEXT[] NOT NULL,
   metrics_upload_at TIMESTAMP WITH TIME ZONE
);

ALTER TABLE workloads
ADD CONSTRAINT fk_workloads_cluster FOREIGN KEY (cluster_id) REFERENCES clusters (id)
ON DELETE CASCADE;

CREATE INDEX idx_workloads_containers ON workloads USING gin(containers);

ALTER TABLE workloads
ADD UNIQUE (org_id, cluster_id, experiment_name);
