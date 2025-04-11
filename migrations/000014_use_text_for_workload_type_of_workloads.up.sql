ALTER TABLE workloads ALTER COLUMN workload_type TYPE TEXT USING workload_type::TEXT;
DROP TYPE IF EXISTS workloadtype;
