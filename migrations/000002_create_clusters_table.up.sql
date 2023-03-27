CREATE TABLE IF NOT EXISTS rosocp.clusters(
   id BIGSERIAL PRIMARY KEY,
   tenant_id BIGINT NOT NULL,
   cluster_uuid TEXT NOT NULL,
   cluster_alias TEXT NOT NULL,
   last_reported_at TIMESTAMP WITH TIME ZONE
);

ALTER TABLE rosocp.clusters
ADD CONSTRAINT fk_clusters_rh_account FOREIGN KEY (tenant_id) REFERENCES rosocp.rh_accounts (id)
ON DELETE CASCADE;

ALTER TABLE rosocp.clusters
ADD UNIQUE (tenant_id, cluster_uuid, cluster_alias);
