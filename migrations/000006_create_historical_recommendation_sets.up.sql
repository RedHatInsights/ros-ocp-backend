CREATE TABLE IF NOT EXISTS historical_recommendation_sets(
   id uuid DEFAULT gen_random_uuid(),
   workload_id uuid NOT NULL,
   cluster_id BIGINT NOT NULL,
   container_name TEXT NOT NULL,
   monitoring_start_time TIMESTAMP WITH TIME ZONE NOT NULL,
   monitoring_end_time TIMESTAMP WITH TIME ZONE NOT NULL,
   recommendations jsonb NOT NULL,
   updated_at TIMESTAMP WITH TIME ZONE NOT NULL
) PARTITION BY RANGE(monitoring_end_time);

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT fk_historical_recommendation_sets_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)
ON DELETE CASCADE;

ALTER TABLE historical_recommendation_sets
ADD CONSTRAINT UQ_historical_recommendation UNIQUE (workload_id, container_name, monitoring_end_time);

CREATE OR REPLACE FUNCTION historical_recommendation_sets_trigger_func() RETURNS trigger AS
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
        partition_table_name = replace('historical_recommendation_sets_' || partition_start_date, '-', '_');

    ELSE
        partition_start_date = CONCAT(record_date, '1');
        partition_end_date = record_date || '16';
        partition_table_name = replace('historical_recommendation_sets_' || partition_start_date, '-', '_');
    END IF;

    IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
        EXECUTE 'CREATE TABLE ' || partition_table_name
            || ' PARTITION OF historical_recommendation_sets FOR VALUES FROM '
            || '(''' || partition_start_date || ''')'
            || ' TO ' 
            || '(''' || partition_end_date || ''')';
        RAISE NOTICE 'A partition has been created %', partition_start_date;

        EXECUTE 'ALTER TABLE ' || partition_table_name
            || ' ADD CONSTRAINT fk_' || partition_table_name || '_workload FOREIGN KEY (workload_id) REFERENCES workloads (id)'
            || ' ON DELETE CASCADE';

        EXECUTE 'ALTER TABLE ' || partition_table_name
            || ' ADD CONSTRAINT UQ_' || partition_table_name || ' UNIQUE (workload_id, container_name, monitoring_end_time)';
    END IF;

    return NEW;
END;
$BODY$
LANGUAGE plpgsql;

CREATE TRIGGER historical_recommendation_sets_insert_trigger BEFORE INSERT ON workloads FOR EACH ROW EXECUTE PROCEDURE historical_recommendation_sets_trigger_func();
