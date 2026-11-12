package operations_test

import (
	"archive/zip"
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/sales"
	"business-manage/internal/modules/treasury"
	"business-manage/internal/operations"
	"business-manage/internal/platform"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOnlineBackupAndRestoreWithWAL(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "data")
	c := operations.Config{Listen: "127.0.0.1:8080", BackupDirectory: "../backups", BackupRetention: 7}
	if err := operations.GenerateCertificates(root, []string{"erp.test"}); err != nil {
		t.Fatal(err)
	}
	if err := operations.WriteConfig(root, c); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.Open(filepath.Join(root, "erp.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(ctx, nil); err != nil {
		t.Fatal(err)
	}
	u := admin.User{ID: core.ID(), Username: "admin", Name: "Admin", Role: "admin", Active: true, PasswordHash: "fixture", CreatedAt: core.Now()}
	err = store.WithinTx(ctx, func(p application.Ports) error {
		if e := p.Admin.CreateCompany(ctx, admin.Company{Name: "Prueba", Currency: "CLP", Timezone: "UTC"}); e != nil {
			return e
		}
		return p.Admin.CreateUser(ctx, u)
	})
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	product, err := app.CreateProduct(ctx, u, catalog.Product{SKU: "P-1", Name: "Producto prueba", Unit: "un", PriceMinor: "1250"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.ReceiveStock(ctx, u, inventory.Receipt{ProductID: product.ID, Quantity: "10", Reason: "Inicial"}); err != nil {
		t.Fatal(err)
	}
	request := sales.Request{Lines: []sales.LineRequest{{ProductID: product.ID, Quantity: "2"}}}
	quote, err := app.QuoteSale(ctx, u, request)
	if err != nil {
		t.Fatal(err)
	}
	request.QuoteFingerprint = quote.Fingerprint
	sale, _, err := app.ConfirmSale(ctx, u, core.Token(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.Exec("INSERT INTO admin_sessions VALUES('token',?,'csrf','2099-01-01T00:00:00Z')", u.ID); err != nil {
		t.Fatal(err)
	}
	printerCfg := pos.Defaults()
	printerCfg.DeviceID = "restored-usb"
	printerCfg.DeviceName = "Impresora respaldada"
	printerCfg.Enabled = true
	printerCfg.DrawerEnabled = true
	printerCfg.PrintVerified = true
	printerCfg.DrawerVerified = true
	printerCfg.Version = 1
	queued := pos.Job{ID: core.ID(), SaleID: sale.ID, Kind: "copy", ActorID: u.ID, CreatedAt: core.Now(), Key: core.Token(), RequestHash: "backup-test", Destination: printerCfg, Payload: []byte("ticket")}
	if err = store.WithinTx(ctx, func(p application.Ports) error {
		if e := p.POS.SaveSettings(ctx, printerCfg, 0); e != nil {
			return e
		}
		return p.POS.Enqueue(ctx, queued)
	}); err != nil {
		t.Fatal(err)
	}
	expense, _, err := app.RecordCashExpense(ctx, u, core.Token(), treasury.ExpenseRequest{AmountMinor: "500", Reason: "Suministros previos al respaldo"})
	if err != nil || expense.DrawerJobID == "" {
		t.Fatal(expense, err)
	}
	archive, err := operations.Backup(ctx, store, root, c)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(t.TempDir(), "other-project")
	os.MkdirAll(foreign, 0700)
	sentinel := filepath.Join(foreign, "important.txt")
	os.WriteFile(sentinel, []byte("keep"), 0600)
	if _, err = operations.Restore(ctx, archive, foreign); err == nil {
		t.Fatal("restored over unrelated directory")
	}
	if contents, readErr := os.ReadFile(sentinel); readErr != nil || string(contents) != "keep" {
		t.Fatal("unrelated directory changed")
	}
	if _, err = app.ReceiveStock(ctx, u, inventory.Receipt{ProductID: product.ID, Quantity: "5", Reason: "Después del respaldo"}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	lock, err := platform.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	previous, err := operations.Restore(ctx, archive, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(previous); err != nil {
		t.Fatal("previous state was not retained", err)
	}
	restored, err := sqlite.Open(filepath.Join(root, "erp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err = restored.Integrity(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := restored.Read().Sales.Sale(ctx, sale.ID)
	if err != nil || got.TotalMinor != "2500" {
		t.Fatalf("sale missing: %+v %v", got, err)
	}
	var stock, sessions int
	if err = restored.DB.QueryRow("SELECT qty_micro/1000 FROM inventory_stock WHERE product_id=?", product.ID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	restored.DB.QueryRow("SELECT count(*) FROM admin_sessions").Scan(&sessions)
	if stock != 8000 || sessions != 0 {
		t.Fatalf("restored stock=%d sessions=%d", stock, sessions)
	}
	job, e := restored.Read().POS.Job(ctx, queued.ID)
	if e != nil || job.State != "suspended" {
		t.Fatal("restored job was not suspended", job, e)
	}
	expenseJob, e := restored.Read().POS.Job(ctx, expense.DrawerJobID)
	if e != nil || expenseJob.State != "suspended" || expenseJob.CashMovementID != expense.ID {
		t.Fatal(expenseJob, e)
	}
	summary, e := application.New(restored).Summary(ctx)
	if e != nil || summary.CashMinor != "2000" || summary.CashExpenseMinor != "500" || summary.RevenueMinor != "2500" {
		t.Fatal(summary, e)
	}
	cfg, e := restored.Read().POS.Settings(ctx)
	if e != nil || cfg.Enabled || cfg.DrawerEnabled || cfg.PrintVerified || cfg.DrawerVerified || cfg.Version <= printerCfg.Version {
		t.Fatal("restored printer was not invalidated", cfg, e)
	}
}
func TestInvalidArchiveDoesNotReplaceExistingData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	os.MkdirAll(root, 0700)
	sentinel := filepath.Join(root, "sentinel")
	os.WriteFile(sentinel, []byte("keep"), 0600)
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(path)
	z := zip.NewWriter(f)
	entry, _ := z.Create("../escape")
	entry.Write([]byte("malicious"))
	z.Close()
	f.Close()
	if _, err := operations.Restore(context.Background(), path, root); err == nil {
		t.Fatal("unsafe archive accepted")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "keep" {
		t.Fatal("existing state modified")
	}
}
func TestProcessLockAndHTTPBinding(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	lock, err := platform.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if second, err := platform.Acquire(root); err == nil {
		second.Close()
		t.Fatal("second writer accepted")
	}
	c := operations.Config{Listen: "0.0.0.0:8080", BackupDirectory: "../backups", BackupRetention: 7}
	if err = c.Validate(); err == nil {
		t.Fatal("LAN HTTP allowed")
	}
	c.TLS = true
	if err = c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.BackupDirectory = "backups"
	if _, err = c.BackupPath(root); err == nil {
		t.Fatal("unsafe backup directory accepted")
	}
}
