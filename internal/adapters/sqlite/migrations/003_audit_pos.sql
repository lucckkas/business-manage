CREATE TABLE catalog_barcodes (
  value TEXT PRIMARY KEY,
  product_id TEXT NOT NULL REFERENCES catalog_products(id),
  format TEXT NOT NULL CHECK(format IN ('CODE128','EAN8','EAN13','UPC')),
  generated INTEGER NOT NULL DEFAULT 0 CHECK(generated IN (0,1)),
  is_primary INTEGER NOT NULL DEFAULT 0 CHECK(is_primary IN (0,1))
) STRICT;
CREATE INDEX catalog_barcodes_product ON catalog_barcodes(product_id);
CREATE UNIQUE INDEX catalog_barcodes_primary ON catalog_barcodes(product_id) WHERE is_primary=1;
CREATE UNIQUE INDEX catalog_barcodes_generated ON catalog_barcodes(product_id) WHERE generated=1;
ALTER TABLE audit_entries ADD COLUMN actor_name TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN reference TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN subject_type TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN subject_id TEXT NOT NULL DEFAULT '';
CREATE INDEX audit_entries_action_date ON audit_entries(action,created_at DESC,id);
CREATE TABLE pos_settings (singleton INTEGER PRIMARY KEY CHECK(singleton=1), config_json TEXT NOT NULL) STRICT;
CREATE TABLE pos_jobs (
  id TEXT PRIMARY KEY,
  sale_id TEXT REFERENCES sales_documents(id),
  kind TEXT NOT NULL CHECK(kind IN ('sale','copy','test_print','test_drawer','drawer')),
  state TEXT NOT NULL CHECK(state IN ('pending','sending','sent','error','uncertain','suspended')),
  actor_id TEXT NOT NULL REFERENCES admin_users(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE,
  request_hash TEXT NOT NULL,
  destination_json TEXT NOT NULL,
  payload BLOB NOT NULL,
  opens_drawer INTEGER NOT NULL CHECK(opens_drawer IN (0,1)),
  reason TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  spool_id TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE UNIQUE INDEX pos_jobs_sale ON pos_jobs(sale_id) WHERE kind='sale';
CREATE INDEX pos_jobs_state_created ON pos_jobs(state,created_at,id);
CREATE INDEX pos_jobs_sale_created ON pos_jobs(sale_id,created_at DESC);
ALTER TABLE sales_documents ADD COLUMN print_job_id TEXT NOT NULL DEFAULT '';
PRAGMA user_version=3;
