CREATE OR REPLACE FUNCTION drop_ros_partition(tableDate TEXT)
RETURNS void AS
$BODY$
DECLARE
    allTables TEXT[];
    tableName TEXT;
BEGIN
    SELECT array_agg(partition_table::TEXT) INTO allTables FROM (SELECT relname AS partition_table, matches[1]::date AS min_rangeval, matches[2]::date AS max_rangeval FROM pg_class CROSS JOIN regexp_matches(pg_get_expr(relpartbound, oid), '\((.+?)\).+\((.+?)\)') AS matches WHERE relispartition AND relkind = 'r') nn WHERE nn.min_rangeval < tableDate::date;

    IF array_length(allTables, 1) > 0 THEN
        FOREACH tableName IN ARRAY allTables
        LOOP
            EXECUTE 'DROP TABLE '||tableName||';';
        END LOOP;
    END IF;
END;
$BODY$
LANGUAGE plpgsql;
