CREATE OR REPLACE FUNCTION drop_ros_partition(tableDate TEXT)
RETURNS void AS
$BODY$
DECLARE
    partitionTables TEXT[];
    partitionTable TEXT;
BEGIN
    -- Below query select all the tables which were created for the date range before $tableDate and capture names of all such tables in $partitionTables[] 
    SELECT array_agg(partition_table::TEXT) INTO partitionTables FROM (SELECT relname AS partition_table, matches[1]::date AS min_rangeval, matches[2]::date AS max_rangeval FROM pg_class CROSS JOIN regexp_matches(pg_get_expr(relpartbound, oid), '\((.+?)\).+\((.+?)\)') AS matches WHERE relispartition AND relkind = 'r') nn WHERE nn.min_rangeval < tableDate::date;

    IF array_length(partitionTables, 1) > 0 THEN
        FOREACH partitionTable IN ARRAY partitionTables
        LOOP
            EXECUTE 'DROP TABLE '||partitionTable||';';
        END LOOP;
    END IF;
END;
$BODY$
LANGUAGE plpgsql;
 