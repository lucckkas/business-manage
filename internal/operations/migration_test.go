package operations_test

import (
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/sales"
	"business-manage/internal/operations"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPriorSchemaMigrationPreservesHistoryAndLegacyRetries(t *testing.T) {
	for _, sourceVersion := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("schema_%d", sourceVersion), func(t *testing.T) {
			ctx := context.Background()
			root := filepath.Join(t.TempDir(), "data")
			if e := operations.GenerateCertificates(root, nil); e != nil {
				t.Fatal(e)
			}
			config := operations.Config{Listen: "127.0.0.1:8080", BackupDirectory: "../backups", BackupRetention: 7}
			if e := operations.WriteConfig(root, config); e != nil {
				t.Fatal(e)
			}
			store, e := sqlite.Open(filepath.Join(root, "erp.db"))
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			ddl, e := os.ReadFile(filepath.Join("..", "adapters", "sqlite", "migrations", "001_initial.sql"))
			if e != nil {
				t.Fatal(e)
			}
			if _, e = store.DB.Exec(string(ddl)); e != nil {
				t.Fatal(e)
			}
			_, e = store.DB.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL); INSERT INTO schema_migrations VALUES(1,?)", core.Now())
			if e != nil {
				t.Fatal(e)
			}
			user, product, sale, receipt := core.ID(), core.ID(), core.ID(), core.ID()
			key := core.Token()
			body := sales.Request{Lines: []sales.LineRequest{{ProductID: product, Quantity: "2.000"}}}
			raw, _ := json.Marshal(body)
			sum := sha256.Sum256([]byte(user + ":" + string(raw)))
			hash := hex.EncodeToString(sum[:])
			now := core.Now()
			statements := []struct {
				sql  string
				args []any
			}{
				{"INSERT INTO admin_company VALUES(1,'Empresa antigua','CLP',0,'UTC')", nil},
				{"INSERT INTO admin_users VALUES(?,?,?,?,?,?,?)", []any{user, "admin", "Admin", "admin", true, "legacy-fixture", now}},
				{"INSERT INTO catalog_products VALUES(?,?,?,?,?,?)", []any{product, "OLD", "Producto anterior", "un", 1250, now}},
				{"INSERT INTO inventory_stock VALUES(?,8000)", []any{product}},
				{"INSERT INTO inventory_receipts VALUES(?,?,10000,'Inicial',?,?)", []any{receipt, product, user, now}},
				{"INSERT INTO sales_documents(number,id,total_minor,actor_id,created_at,idempotency_key,request_hash) VALUES(42,?,2500,?,?,?,?)", []any{sale, user, now, key, hash}},
				{"INSERT INTO sales_lines VALUES(?,?,?,'Producto anterior','un',2000,1250,2500)", []any{sale, product, "OLD"}},
				{"INSERT INTO inventory_movements(id,product_id,qty_milli,receipt_id,actor_id,created_at) VALUES(?,?,10000,?,?,?)", []any{core.ID(), product, receipt, user, now}},
				{"INSERT INTO inventory_movements(id,product_id,qty_milli,sale_id,actor_id,created_at) VALUES(?,?,-2000,?,?,?)", []any{core.ID(), product, sale, user, now}},
				{"INSERT INTO treasury_movements VALUES(?,?,2500,?,?)", []any{core.ID(), sale, user, now}},
				{"INSERT INTO audit_entries VALUES(?,?,'sale.confirm',?,?)", []any{core.ID(), user, sale, now}},
			}
			for _, statement := range statements {
				if _, e = store.DB.Exec(statement.sql, statement.args...); e != nil {
					t.Fatal(e)
				}
			}
			if sourceVersion >= 2 {
				ddl, e := os.ReadFile(filepath.Join("..", "adapters", "sqlite", "migrations", "002_commercial.sql"))
				if e != nil {
					t.Fatal(e)
				}
				if _, e = store.DB.Exec(string(ddl)); e != nil {
					t.Fatal(e)
				}
				if _, e = store.DB.Exec("INSERT INTO schema_migrations VALUES(2,?)", core.Now()); e != nil {
					t.Fatal(e)
				}
			}
			oldJobID := ""
			if sourceVersion == 3 {
				ddl, e := os.ReadFile(filepath.Join("..", "adapters", "sqlite", "migrations", "003_audit_pos.sql"))
				if e != nil {
					t.Fatal(e)
				}
				if _, e = store.DB.Exec(string(ddl)); e != nil {
					t.Fatal(e)
				}
				if _, e = store.DB.Exec("INSERT INTO schema_migrations VALUES(3,?)", core.Now()); e != nil {
					t.Fatal(e)
				}
				oldJobID = core.ID()
				if _, e = store.DB.Exec("INSERT INTO pos_jobs(id,sale_id,kind,state,actor_id,created_at,updated_at,idempotency_key,request_hash,destination_json,payload,opens_drawer) VALUES(?,?,'sale','pending',?,?,?,?,?,'{}',?,1)", oldJobID, sale, user, now, now, core.Token(), "old-job", []byte("original ESC/POS")); e != nil {
					t.Fatal(e)
				}
			}
			backed := false
			if e = store.Migrate(ctx, func() error { _, e := operations.Backup(ctx, store, root, config); backed = e == nil; return e }); e != nil {
				t.Fatal(e)
			}
			if !backed {
				t.Fatal("no pre-migration backup")
			}
			if e = store.Integrity(ctx); e != nil {
				t.Fatal(e)
			}
			balance, e := store.Read().Inventory.Stock(ctx, product)
			if e != nil || balance != 8_000_000 {
				t.Fatalf("stock=%d %v", balance, e)
			}
			movements, e := store.Read().Reports.Cash(ctx, core.Query{})
			if e != nil || movements.Total != 1 || movements.Items[0].AmountMinor != "2500" || movements.Items[0].SaleID != sale {
				t.Fatal(movements, e)
			}
			if oldJobID != "" {
				job, e := store.Read().POS.Job(ctx, oldJobID)
				if e != nil || job.SaleID != sale || job.State != "pending" || string(job.Payload) != "original ESC/POS" || !job.OpensDrawer {
					t.Fatal("schema 3 job changed", job, e)
				}
			}
			app := application.New(store)
			actor := admin.User{ID: user, Role: "admin", Active: true}
			previous, replayed, e := app.ConfirmSale(ctx, actor, key, body)
			if e != nil || !replayed || previous.Number != 42 || previous.TotalMinor != "2500" || previous.PromotionDiscountMinor != "0" || previous.Company.Name != "Empresa antigua" {
				t.Fatalf("legacy retry %+v %v %v", previous, replayed, e)
			}
			body.Lines[0].Quantity = "1"
			q, e := app.QuoteSale(ctx, actor, body)
			if e != nil {
				t.Fatal(e)
			}
			body.QuoteFingerprint = q.Fingerprint
			next, _, e := app.ConfirmSale(ctx, actor, core.Token(), body)
			if e != nil || next.Number != 43 {
				t.Fatalf("correlative %+v %v", next, e)
			}
		})
	}
}
