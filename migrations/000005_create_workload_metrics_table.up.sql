CREATE TABLE IF NOT EXISTS workload_metrics(
   id uuid DEFAULT gen_random_uuid(),
   workload_id uuid NOT NULL,
   cluster_id BIGINT NOT NULL,
   container_name TEXT NOT NULL,
   interval_start TIMESTAMP WITH TIME ZONE NOT NULL,
   interval_end TIMESTAMP WITH TIME ZONE NOT NULL,
   usage_metrics jsonb NOT NULL
) PARTITION BY RANGE(interval_end);

ALTER TABLE workload_metrics
ADD CONSTRAINT fk_workload_metrics_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE workload_metrics
ADD CONSTRAINT UQ_Workload_Metrics UNIQUE (workload_id, container_name, interval_start, interval_end);

CREATE OR REPLACE FUNCTION workload_metrics_insert_trigger_func() RETURNS trigger AS
$BODY$
DECLARE
    record_day INT;
    record_date TEXT;
    partition_start_date TEXT;
    partition_end_date TEXT;
    partition_table_name TEXT; 
    end_of_month TIMESTAMP;
BEGIN
    record_day := TO_NUMBER(TO_CHAR(NEW.metrics_upload_at,'DD'),'99');
    record_date := TO_CHAR(NEW.metrics_upload_at,'YYYY-MM-');
    select (date_trunc('month', NEW.metrics_upload_at) + interval '1 month - 1 day')::date INTO end_of_month;
    IF record_day > 15 THEN
        partition_start_date = CONCAT(record_date, '16');
        partition_end_date = end_of_month;
        partition_table_name = replace('workload_metrics_' || partition_start_date, '-', '_');

    ELSE
        partition_start_date = CONCAT(record_date, '1');
        partition_end_date = record_date || '16';
        partition_table_name = replace('workload_metrics_' || partition_start_date, '-', '_');
    END IF;

    IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
        EXECUTE 'CREATE TABLE ' || partition_table_name
            || ' PARTITION OF workload_metrics FOR VALUES FROM '
            || '(''' || partition_start_date || ''')'
            || ' TO ' 
            || '(''' || partition_end_date || ''')';

        EXECUTE 'ALTER TABLE ' || partition_table_name
            || ' ADD CONSTRAINT fk_' || partition_table_name || '_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)'
            || ' ON DELETE CASCADE';
        
        EXECUTE 'ALTER TABLE ' || partition_table_name
            || ' ADD CONSTRAINT UQ_' || partition_table_name || ' UNIQUE (workload_id, container_name, interval_start, interval_end)';
    END IF;

    return NEW;
END;
$BODY$
LANGUAGE plpgsql;

CREATE TRIGGER workload_metrics_insert_trigger BEFORE INSERT ON workloads FOR EACH ROW EXECUTE PROCEDURE workload_metrics_insert_trigger_func();
