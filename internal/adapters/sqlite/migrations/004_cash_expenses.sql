ALTER TABLE admin_users ADD COLUMN can_cash_expense INTEGER NOT NULL DEFAULT 0 CHECK(can_cash_expense IN (0,1));

CREATE TABLE treasury_movements_v4 (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK(kind IN ('sale','expense')),
  sale_id TEXT UNIQUE REFERENCES sales_documents(id),
  expense_number INTEGER UNIQUE CHECK(expense_number>0),
  amount_minor INTEGER NOT NULL CHECK(amount_minor BETWEEN -1000000000000000 AND 1000000000000000),
  actor_id TEXT NOT NULL REFERENCES admin_users(id),
  actor_name TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  balance_before TEXT,
  balance_after TEXT,
  idempotency_key TEXT UNIQUE,
  request_hash TEXT NOT NULL DEFAULT '',
  drawer_job_id TEXT NOT NULL DEFAULT '',
  CHECK((kind='sale' AND sale_id IS NOT NULL AND expense_number IS NULL AND amount_minor>=0 AND idempotency_key IS NULL)
     OR (kind='expense' AND sale_id IS NULL AND expense_number IS NOT NULL AND amount_minor<0 AND length(reason) BETWEEN 3 AND 240 AND idempotency_key IS NOT NULL AND balance_before IS NOT NULL AND balance_after IS NOT NULL))
) STRICT;
INSERT INTO treasury_movements_v4(id,kind,sale_id,amount_minor,actor_id,created_at)
SELECT id,'sale',sale_id,amount_minor,actor_id,created_at FROM treasury_movements;
DROP TABLE treasury_movements;
ALTER TABLE treasury_movements_v4 RENAME TO treasury_movements;
CREATE INDEX treasury_movements_date ON treasury_movements(created_at DESC,id);
CREATE INDEX treasury_movements_kind_date ON treasury_movements(kind,created_at DESC,id);
CREATE TRIGGER treasury_no_update BEFORE UPDATE ON treasury_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;
CREATE TRIGGER treasury_no_delete BEFORE DELETE ON treasury_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;

CREATE TABLE pos_jobs_v4 (
  id TEXT PRIMARY KEY,
  sale_id TEXT REFERENCES sales_documents(id),
  cash_movement_id TEXT REFERENCES treasury_movements(id),
  kind TEXT NOT NULL CHECK(kind IN ('sale','copy','test_print','test_drawer','drawer','expense_drawer')),
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
  spool_id TEXT NOT NULL DEFAULT '',
  CHECK((kind='expense_drawer' AND cash_movement_id IS NOT NULL AND sale_id IS NULL AND opens_drawer=1)
     OR (kind!='expense_drawer' AND cash_movement_id IS NULL))
) STRICT;
INSERT INTO pos_jobs_v4(id,sale_id,kind,state,actor_id,created_at,updated_at,idempotency_key,request_hash,destination_json,payload,opens_drawer,reason,error,spool_id)
SELECT id,sale_id,kind,state,actor_id,created_at,updated_at,idempotency_key,request_hash,destination_json,payload,opens_drawer,reason,error,spool_id FROM pos_jobs;
DROP TABLE pos_jobs;
ALTER TABLE pos_jobs_v4 RENAME TO pos_jobs;
CREATE UNIQUE INDEX pos_jobs_sale ON pos_jobs(sale_id) WHERE kind='sale';
CREATE UNIQUE INDEX pos_jobs_expense ON pos_jobs(cash_movement_id) WHERE kind='expense_drawer';
CREATE INDEX pos_jobs_state_created ON pos_jobs(state,created_at,id);
CREATE INDEX pos_jobs_sale_created ON pos_jobs(sale_id,created_at DESC);
PRAGMA user_version=4;
