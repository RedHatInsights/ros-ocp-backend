CREATE OR REPLACE FUNCTION create_range_patition(partition_table_name TEXT, parent_table Text, partition_start_date Text, partition_end_date Text)
RETURNS void AS
$BODY$
DECLARE
BEGIN
   EXECUTE 'CREATE TABLE ' || partition_table_name
      || ' PARTITION OF '|| parent_table ||' FOR VALUES FROM '
      || '(''' || partition_start_date || ''')'
      || ' TO ' 
      || '(''' || partition_end_date || ''')';
END;
$BODY$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION create_monthly_patitions(max_interval_end TIMESTAMP WITH TIME ZONE, parent_table Text)
RETURNS void AS
$BODY$
DECLARE
   record_day INT;
   record_date TEXT;
   partition_start_date TEXT;
   partition_end_date TEXT;
   partition_table_name TEXT; 
BEGIN
    record_day := TO_NUMBER(TO_CHAR(max_interval_end,'DD'),'99');
    record_date := TO_CHAR(max_interval_end,'YYYY-MM-');
    IF record_day > 15 THEN
        partition_start_date = CONCAT(record_date, '16');
        select (date_trunc('month', max_interval_end) + interval '1 month - 1 day')::date INTO partition_end_date;
        partition_table_name = replace(parent_table || '_' || partition_start_date, '-', '_');
        IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
            EXECUTE create_range_patition(partition_table_name, parent_table, partition_start_date, partition_end_date);
        END IF;

        partition_start_date = CONCAT(record_date, '1');
        partition_end_date = record_date || '16';
        partition_table_name = replace(parent_table || '_' || partition_start_date, '-', '_');
        IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
            EXECUTE create_range_patition(partition_table_name, parent_table, partition_start_date, partition_end_date);
        END IF;
    ELSE
        partition_start_date = CONCAT(record_date, '1');
        partition_end_date = record_date || '16';
        partition_table_name = replace(parent_table || '_' || partition_start_date, '-', '_');
        IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
            EXECUTE create_range_patition(partition_table_name, parent_table, partition_start_date, partition_end_date);
        END IF;

        select (date_trunc('month', max_interval_end) - interval '1 month' + interval '15 days' )::date INTO partition_start_date;
        select (date_trunc('month', max_interval_end))::date INTO partition_end_date;
        partition_table_name = replace(parent_table || '_' || partition_start_date, '-', '_');
        IF NOT EXISTS(SELECT relname FROM pg_class WHERE relname=partition_table_name) THEN
            EXECUTE create_range_patition(partition_table_name, parent_table, partition_start_date, partition_end_date);
        END IF;
    END IF;
END;
$BODY$
LANGUAGE plpgsql;
