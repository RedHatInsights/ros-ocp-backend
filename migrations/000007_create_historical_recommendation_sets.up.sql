CREATE TABLE IF NOT EXISTS historical_recommendation_sets(
   id BIGSERIAL NOT NULL,
   org_id TEXT NOT NULL,
   workload_id BIGINT NOT NULL,
   container_name TEXT NOT NULL,
   monitoring_start_time TIMESTAMP WITH TIME ZONE NOT NULL,
   monitoring_end_time TIMESTAMP WITH TIME ZONE NOT NULL,
   recommendations jsonb NOT NULL,
   updated_at TIMESTAMP WITH TIME ZONE NOT NULL
) PARTITION BY LIST(org_id);

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT fk_historical_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT UQ_historical_recommendation UNIQUE (org_id, workload_id, container_name, monitoring_end_time);

CREATE OR REPLACE FUNCTION historical_recommendation_sets_insert_trigger_func() RETURNS trigger AS
$BODY$
DECLARE
    org_id_partition_table_name TEXT;
BEGIN
    org_id_partition_table_name := 'historical_recommendation_sets_' || New.org_id;
    IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=org_id_partition_table_name) THEN
        EXECUTE 'CREATE TABLE ' || org_id_partition_table_name
                || ' PARTITION OF historical_recommendation_sets FOR VALUES IN'
                || ' (''' || NEW.org_id || ''')'
                || ' PARTITION BY RANGE(monitoring_end_time)';
    END IF;
    EXECUTE create_monthly_partitions(NEW.metrics_upload_at, org_id_partition_table_name);
    return NEW;
END;
$BODY$
LANGUAGE plpgsql;

CREATE TRIGGER historical_recommendation_sets_insert_trigger BEFORE INSERT ON workloads FOR EACH ROW EXECUTE PROCEDURE historical_recommendation_sets_insert_trigger_func();
