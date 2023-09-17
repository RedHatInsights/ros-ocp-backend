CREATE TABLE IF NOT EXISTS workload_metrics(
   id BIGSERIAL NOT NULL,
   org_id TEXT NOT NULL,
   workload_id BIGINT NOT NULL,
   container_name TEXT NOT NULL,
   interval_start TIMESTAMP WITH TIME ZONE NOT NULL,
   interval_end TIMESTAMP WITH TIME ZONE NOT NULL,
   usage_metrics jsonb NOT NULL
) PARTITION BY LIST(org_id);

ALTER TABLE workload_metrics
ADD CONSTRAINT fk_workload_metrics_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE workload_metrics
ADD CONSTRAINT UQ_Workload_Metrics UNIQUE (org_id, workload_id, container_name, interval_start, interval_end);


CREATE OR REPLACE FUNCTION workload_metrics_insert_trigger_func() RETURNS trigger AS
$BODY$
DECLARE
   org_id_partition_table_name TEXT;
BEGIN
   org_id_partition_table_name := 'workload_metrics_' || New.org_id;
   IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=org_id_partition_table_name) THEN
      EXECUTE 'CREATE TABLE ' || org_id_partition_table_name
            || ' PARTITION OF workload_metrics FOR VALUES IN'
            || ' (''' || NEW.org_id || ''')'
            || ' PARTITION BY RANGE(interval_end)';
   END IF;
   EXECUTE create_monthly_patitions(NEW.metrics_upload_at, org_id_partition_table_name);
   return NEW;
END;
$BODY$
LANGUAGE plpgsql;

CREATE TRIGGER workload_metrics_insert_trigger BEFORE INSERT ON workloads FOR EACH ROW EXECUTE PROCEDURE workload_metrics_insert_trigger_func();
