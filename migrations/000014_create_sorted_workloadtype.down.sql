CREATE TYPE workloadtype AS ENUM ('daemonset', 'deployment', 'deploymentconfig', 'replicaset', 'replicationcontroller', 'statefulset');
ALTER TABLE workloads ALTER COLUMN workload_type type workloadtype USING workload_type::text::workloadtype;
DROP TYPE IF EXISTS sorted_workloadtype;
