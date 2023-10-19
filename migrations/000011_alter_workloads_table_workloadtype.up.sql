ALTER TYPE workloadtype RENAME TO workloadtype_old;

CREATE TYPE workloadtype AS ENUM ('daemonset', 'deployment', 'deploymentconfig', 'replicaset', 'replicationcontroller', 'statefulset');

ALTER TABLE workloads ALTER COLUMN workload_type TYPE VARCHAR;

DROP TYPE workloadtype_old;

ALTER TABLE workloads ALTER COLUMN workload_type TYPE workloadtype USING workload_type::workloadtype;
