CREATE TABLE admin_company (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1), name TEXT NOT NULL,
  currency TEXT NOT NULL, currency_scale INTEGER NOT NULL CHECK(currency_scale BETWEEN 0 AND 3), timezone TEXT NOT NULL
) STRICT;
CREATE TABLE admin_users (
  id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('admin','operator')), active INTEGER NOT NULL CHECK(active IN (0,1)),
  password_hash TEXT NOT NULL, created_at TEXT NOT NULL
) STRICT;
CREATE TABLE admin_sessions (
  hash TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES admin_users(id), csrf_token TEXT NOT NULL, expires_at TEXT NOT NULL
) STRICT;
CREATE INDEX admin_sessions_user ON admin_sessions(user_id);
CREATE INDEX admin_sessions_expiry ON admin_sessions(expires_at);
CREATE TABLE catalog_products (
  id TEXT PRIMARY KEY, sku TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
  unit TEXT NOT NULL CHECK(unit IN ('un','kg','lt','mt')), price_minor INTEGER NOT NULL CHECK(price_minor BETWEEN 0 AND 1000000000000000), created_at TEXT NOT NULL
) STRICT;
CREATE INDEX catalog_products_name ON catalog_products(name COLLATE NOCASE);
CREATE TABLE inventory_stock (
  product_id TEXT PRIMARY KEY REFERENCES catalog_products(id),
  qty_milli INTEGER NOT NULL CHECK(qty_milli BETWEEN 0 AND 1000000000000)
) STRICT;
CREATE TABLE inventory_receipts (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL REFERENCES catalog_products(id),
  qty_milli INTEGER NOT NULL CHECK(qty_milli>0), reason TEXT NOT NULL,
  actor_id TEXT NOT NULL REFERENCES admin_users(id), created_at TEXT NOT NULL
) STRICT;
CREATE TABLE sales_documents (
  number INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE,
  total_minor INTEGER NOT NULL CHECK(total_minor BETWEEN 0 AND 1000000000000000),
  actor_id TEXT NOT NULL REFERENCES admin_users(id), created_at TEXT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE, request_hash TEXT NOT NULL
) STRICT;
CREATE INDEX sales_documents_date ON sales_documents(created_at DESC,number DESC);
CREATE TABLE sales_lines (
  sale_id TEXT NOT NULL REFERENCES sales_documents(id), product_id TEXT NOT NULL REFERENCES catalog_products(id),
  sku TEXT NOT NULL, name TEXT NOT NULL, unit TEXT NOT NULL, qty_milli INTEGER NOT NULL CHECK(qty_milli>0),
  price_minor INTEGER NOT NULL CHECK(price_minor>=0), total_minor INTEGER NOT NULL CHECK(total_minor>=0),
  PRIMARY KEY(sale_id,product_id)
) STRICT;
CREATE TABLE inventory_movements (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL REFERENCES catalog_products(id), qty_milli INTEGER NOT NULL CHECK(qty_milli!=0),
  receipt_id TEXT REFERENCES inventory_receipts(id), sale_id TEXT REFERENCES sales_documents(id),
  actor_id TEXT NOT NULL REFERENCES admin_users(id), created_at TEXT NOT NULL,
  CHECK((receipt_id IS NOT NULL AND sale_id IS NULL AND qty_milli>0) OR (sale_id IS NOT NULL AND receipt_id IS NULL AND qty_milli<0)),
  UNIQUE(sale_id,product_id), UNIQUE(receipt_id)
) STRICT;
CREATE INDEX inventory_movements_product_date ON inventory_movements(product_id,created_at);
CREATE TABLE treasury_movements (
  id TEXT PRIMARY KEY, sale_id TEXT NOT NULL UNIQUE REFERENCES sales_documents(id),
  amount_minor INTEGER NOT NULL CHECK(amount_minor>=0), actor_id TEXT NOT NULL REFERENCES admin_users(id), created_at TEXT NOT NULL
) STRICT;
CREATE INDEX treasury_movements_date ON treasury_movements(created_at DESC);
CREATE TABLE audit_entries (
  id TEXT PRIMARY KEY, actor_id TEXT NOT NULL, action TEXT NOT NULL, entity_id TEXT NOT NULL, created_at TEXT NOT NULL
) STRICT;
CREATE INDEX audit_entries_date ON audit_entries(created_at DESC);
CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit_entries BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit_entries BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER inventory_no_update BEFORE UPDATE ON inventory_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;
CREATE TRIGGER inventory_no_delete BEFORE DELETE ON inventory_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;
CREATE TRIGGER treasury_no_update BEFORE UPDATE ON treasury_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;
CREATE TRIGGER treasury_no_delete BEFORE DELETE ON treasury_movements BEGIN SELECT RAISE(ABORT,'ledger is append-only'); END;
PRAGMA user_version=1;
