CREATE TYPE sorted_workloadtype AS ENUM ('daemonset', 'deployment', 'deploymentconfig', 'replicaset', 'replicationcontroller', 'statefulset');
ALTER TABLE workloads ALTER COLUMN workload_type type sorted_workloadtype USING workload_type::text::sorted_workloadtype;
DROP TYPE IF EXISTS workloadtype;
