DELETE FROM recommendation_sets WHERE id IN 
(SELECT id FROM recommendation_sets r1 JOIN 
(SELECT workload_id, container_name, MAX(monitoring_end_time) AS monitoring_end_time FROM recommendation_sets GROUP BY workload_id, container_name) AS r2 
ON r1.workload_id = r2.workload_id AND r1.container_name = r2.container_name AND r1.monitoring_end_time != r2.monitoring_end_time);
