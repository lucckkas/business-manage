package application_test

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/pos"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

type fakePrinter struct {
	mu      sync.Mutex
	writes  [][]byte
	failure error
	entered chan struct{}
	release chan struct{}
}

func (f *fakePrinter) Devices(context.Context) ([]pos.Device, error) {
	return []pos.Device{{ID: "usb-test", Name: "Xprinter de prueba", Transport: "fake"}}, nil
}
func (f *fakePrinter) Send(_ context.Context, _ string, payload []byte) (string, error) {
	f.mu.Lock()
	f.writes = append(f.writes, append([]byte{}, payload...))
	failure, entered, release := f.failure, f.entered, f.release
	f.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if release != nil {
		<-release
	}
	return "test-spool", failure
}
func readyPrinter(t *testing.T, f *fixture) (*fakePrinter, pos.Settings) {
	t.Helper()
	printer := &fakePrinter{}
	f.app.Printer = printer
	cfg := pos.Defaults()
	old, err := f.store.Read().POS.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Version = old.Version
	cfg.DeviceID = "usb-test"
	cfg, e := f.app.SavePOSSettings(ctx, f.user, cfg)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"test_print", "test_drawer"} {
		j, _, e := f.app.RequestPrint(ctx, f.user, core.Token(), kind, "", "")
		if e != nil {
			t.Fatal(e)
		}
		if worked, e := f.app.ProcessPrintJob(ctx); e != nil || !worked {
			t.Fatal(e)
		}
		cfg, e = f.app.VerifyPOSTest(ctx, f.user, j.ID)
		if e != nil {
			t.Fatal(e)
		}
	}
	cfg.Enabled = true
	cfg.DrawerEnabled = true
	cfg, e = f.app.SavePOSSettings(ctx, f.user, cfg)
	if e != nil {
		t.Fatal(e)
	}
	return printer, cfg
}
func TestBarcodesAreUniqueStableExactAndAudited(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "BAR-1", "100", "4")
	p.Barcodes = []catalog.Barcode{{Value: "000123456789", Format: "CODE128", Primary: true}, {Value: "4006381333931", Format: "EAN13"}}
	p, e := f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p})
	if e != nil {
		t.Fatal(e)
	}
	found, e := f.store.Read().Catalog.ProductByBarcode(ctx, "000123456789")
	if e != nil || found.ID != p.ID {
		t.Fatal(found, e)
	}
	if _, e = f.store.Read().Catalog.ProductByBarcode(ctx, "123456789"); !core.HasCode(e, "not_found") {
		t.Fatal("leading zero was lost")
	}
	q := f.product(t, "BAR-2", "100", "1")
	q.Barcodes = []catalog.Barcode{{Value: p.Barcodes[0].Value, Format: "CODE128"}}
	if _, e = f.app.UpdateProduct(ctx, f.user, q.ID, application.ProductUpdate{Product: q}); !core.HasCode(e, "conflict") {
		t.Fatal("duplicate barcode accepted", e)
	}
	generated := make(chan string, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := f.app.GenerateBarcode(ctx, f.user, p.ID)
			if e != nil {
				t.Error(e)
				return
			}
			for _, b := range v.Barcodes {
				if b.Generated {
					generated <- b.Value
				}
			}
		}()
	}
	wg.Wait()
	close(generated)
	value := ""
	for v := range generated {
		if value != "" && value != v {
			t.Fatal("unstable generated code")
		}
		value = v
	}
	if value == "" {
		t.Fatal("no generated code")
	}
	var count int
	f.store.DB.QueryRow("SELECT count(*) FROM audit_entries WHERE action='product.barcode_generate'").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate generation events", count)
	}
	p, _ = f.store.Read().Catalog.Product(ctx, p.ID)
	if len(p.Barcodes) != 3 {
		t.Fatal(p.Barcodes)
	}
	for i := range p.Barcodes {
		p.Barcodes[i].Generated = false
	}
	p, e = f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p})
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.app.GenerateBarcode(ctx, f.user, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range again.Barcodes {
		if b.Generated && b.Value != value {
			t.Fatal("ordinary edit changed internal identity")
		}
	}
	if _, e = f.app.GenerateBarcode(ctx, admin.User{Role: "operator"}, p.ID); !core.HasCode(e, "forbidden") {
		t.Fatal(e)
	}
}
func TestAuditSearchDetailsAndActorSnapshots(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "AUD-1", "100", "3")
	p.Name = "Nombre registrado"
	p.PriceMinor = "125"
	if _, e := f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p}); e != nil {
		t.Fatal(e)
	}
	page, e := f.store.Read().Reports.Audit(ctx, core.Query{Search: "ED", PageSize: 1})
	if e != nil || page.Total != 1 || page.Items[0].Action != "product.update" {
		t.Fatalf("search %+v %v", page, e)
	}
	entry := page.Items[0]
	if entry.Reference != "Producto AUD-1" || entry.ActorName != f.user.Name || entry.Summary != "Nombre registrado" {
		t.Fatal(entry)
	}
	if _, e = f.app.UpdateProfile(ctx, f.user, "Nombre posterior"); e != nil {
		t.Fatal(e)
	}
	saved, e := f.store.Read().Audit.Entry(ctx, entry.ID)
	if e != nil || saved.ActorName != f.user.Name {
		t.Fatal("actor snapshot changed", saved, e)
	}
	raw, _ := json.Marshal(saved.Details)
	if !bytes.Contains(raw, []byte("Producto AUD-1")) || !bytes.Contains(raw, []byte("Nombre registrado")) {
		t.Fatal(string(raw))
	}
	accented, e := f.store.Read().Reports.Audit(ctx, core.Query{Search: "edición", Action: "product.update"})
	if e != nil || accented.Total != 1 {
		t.Fatal(accented, e)
	}
	if e = f.store.WithinTx(ctx, func(p application.Ports) error {
		return p.Audit.Append(ctx, audit.Entry{ID: core.ID(), ActorID: f.user.ID, Action: "user.profile", EntityID: f.user.ID, CreatedAt: core.Now(), Details: map[string]any{"passwordHash": "PRIVATE_HASH_VALUE", "nested": map[string]any{"csrfToken": "PRIVATE_TOKEN_VALUE"}, "name": "Dato permitido"}})
	}); e != nil {
		t.Fatal(e)
	}
	events, _ := f.store.Read().Reports.Audit(ctx, core.Query{Action: "user.profile"})
	for _, a := range events.Items {
		raw, _ = json.Marshal(a)
		if bytes.Contains(raw, []byte("PRIVATE_")) {
			t.Fatal("sensitive audit content exposed")
		}
	}
}
func TestAutomaticPrinterJobsAreAtomicAndIdempotent(t *testing.T) {
	f := newFixture(t)
	printer, _ := readyPrinter(t, f)
	p := f.product(t, "PRINT", "1250", "5")
	key := core.Token()
	req := request(p.ID, "2")
	sale, _, e := f.confirm(ctx, f.user, key, req)
	if e != nil || sale.PrintJobID == "" {
		t.Fatal(sale, e)
	}
	if len(printer.writes) != 2 {
		t.Fatal("hardware contacted inside sale transaction")
	}
	if _, replayed, e := f.app.ConfirmSale(ctx, f.user, key, req); e != nil || !replayed {
		t.Fatal("sale retry failed", e)
	}
	var jobs int
	f.store.DB.QueryRow("SELECT count(*) FROM pos_jobs WHERE sale_id=? AND kind='sale'", sale.ID).Scan(&jobs)
	if jobs != 1 {
		t.Fatal(jobs)
	}
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || !worked {
		t.Fatal(e)
	}
	output := printer.writes[len(printer.writes)-1]
	if bytes.Count(output, []byte{27, 'p'}) != 1 || !bytes.Contains(output, []byte("2500")) {
		t.Fatal("wrong sale ticket", output)
	}
	printKey := core.Token()
	copy, _, e := f.app.RequestPrint(ctx, f.user, printKey, "copy", sale.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if again, replayed, e := f.app.RequestPrint(ctx, f.user, printKey, "copy", sale.ID, ""); e != nil || !replayed || again.ID != copy.ID {
		t.Fatal("copy retry duplicated", e)
	}
	if _, e = f.app.ProcessPrintJob(ctx); e != nil {
		t.Fatal(e)
	}
	output = printer.writes[len(printer.writes)-1]
	if bytes.Contains(output, []byte{27, 'p'}) || !bytes.Contains(output, []byte("COPIA")) {
		t.Fatal("copy opened drawer")
	}
	if stock(t, f, p.ID) != 3000 || count(t, f.store, "sales_documents") != 1 {
		t.Fatal("printing changed stock or sale")
	}
}
func TestPrinterFailuresAndRecoveryNeverResendUncertainOutput(t *testing.T) {
	f := newFixture(t)
	printer, _ := readyPrinter(t, f)
	p := f.product(t, "FAIL-PRINT", "100", "2")
	sale, _, e := f.confirm(ctx, f.user, core.Token(), request(p.ID, "1"))
	if e != nil {
		t.Fatal(e)
	}
	printer.failure = &pos.SendError{Err: errors.New("partial USB write"), Uncertain: true}
	if _, e = f.app.ProcessPrintJob(ctx); e != nil {
		t.Fatal(e)
	}
	job, _ := f.store.Read().POS.Job(ctx, sale.PrintJobID)
	if job.State != "uncertain" {
		t.Fatal(job)
	}
	calls := len(printer.writes)
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || worked || len(printer.writes) != calls {
		t.Fatal("uncertain work retried")
	}
	if count(t, f.store, "sales_documents") != 1 || stock(t, f, p.ID) != 1000 {
		t.Fatal("printer failure rolled back sale")
	}
	printer.failure = nil
	j, _, e := f.app.RequestPrint(ctx, f.user, core.Token(), "copy", sale.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = f.store.WithinTx(ctx, func(p application.Ports) error { _, e := p.POS.Claim(ctx); return e }); e != nil {
		t.Fatal(e)
	}
	if e = f.app.RecoverPrintJobs(ctx); e != nil {
		t.Fatal(e)
	}
	j, _ = f.store.Read().POS.Job(ctx, j.ID)
	if j.State != "uncertain" || len(printer.writes) != calls {
		t.Fatal("interrupted output was resent")
	}
}
func TestPrinterConfigurationPermissionsAndTransactionalRollback(t *testing.T) {
	f := newFixture(t)
	f.app.Printer = &fakePrinter{}
	cfg := pos.Defaults()
	cfg.DeviceID = "usb-test"
	cfg.Enabled = true
	cfg.PrintVerified = true
	if _, e := f.app.SavePOSSettings(ctx, f.user, cfg); e == nil {
		t.Fatal("client forged physical verification")
	}
	printer, cfg := readyPrinter(t, f)
	if _, _, e := f.app.RequestPrint(ctx, admin.User{Role: "operator"}, core.Token(), "drawer", "", "Abrir caja"); !core.HasCode(e, "forbidden") {
		t.Fatal(e)
	}
	if _, _, e := f.app.RequestPrint(ctx, f.user, core.Token(), "drawer", "", ""); e == nil {
		t.Fatal("reasonless drawer opening")
	}
	key := core.Token()
	j, _, e := f.app.RequestPrint(ctx, f.user, key, "drawer", "", "Retiro autorizado")
	if e != nil {
		t.Fatal(e)
	}
	again, replayed, e := f.app.RequestPrint(ctx, f.user, key, "drawer", "", "Retiro autorizado")
	if e != nil || !replayed || again.ID != j.ID {
		t.Fatal("duplicate drawer request")
	}
	if _, e = f.app.ProcessPrintJob(ctx); e != nil {
		t.Fatal(e)
	}
	cfg.Width = 58
	cfg.Enabled = false
	cfg.DrawerEnabled = false
	cfg, e = f.app.SavePOSSettings(ctx, f.user, cfg)
	if e != nil || cfg.PrintVerified || cfg.DrawerVerified {
		t.Fatal("profile change kept verification", e)
	}
	// A late audit failure must undo the sale and its queued peripheral work.
	_, _ = readyPrinter(t, f)
	p := f.product(t, "ROLL-PRINT", "100", "2")
	before := count(t, f.store, "pos_jobs")
	calls := len(printer.writes)
	if _, e = f.store.DB.Exec("CREATE TRIGGER reject_pos_audit BEFORE INSERT ON audit_entries WHEN NEW.action='pos.enqueue' BEGIN SELECT RAISE(ABORT,'injected'); END"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.confirm(ctx, f.user, core.Token(), request(p.ID, "1")); e == nil {
		t.Fatal("injected failure missing")
	}
	if count(t, f.store, "sales_documents") != 0 || count(t, f.store, "pos_jobs") != before || stock(t, f, p.ID) != 2000 || len(printer.writes) != calls {
		t.Fatal("partial sale or output survived failure")
	}
}

func TestPrinterSerializesWorkersAndRejectsStalePhysicalTests(t *testing.T) {
	f := newFixture(t)
	printer, cfg := readyPrinter(t, f)
	first, _, e := f.app.RequestPrint(ctx, f.user, core.Token(), "test_print", "", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.app.RequestPrint(ctx, f.user, core.Token(), "test_print", "", ""); e != nil {
		t.Fatal(e)
	}
	printer.entered = make(chan struct{}, 1)
	printer.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { _, e := f.app.ProcessPrintJob(ctx); done <- e }()
	<-printer.entered
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || worked {
		t.Fatal("parallel output was claimed", e)
	}
	close(printer.release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || !worked {
		t.Fatal("next queued job was not dispatched", e)
	}
	cfg.Enabled = false
	cfg.DrawerEnabled = false
	cfg.Width = 58
	if _, e = f.app.SavePOSSettings(ctx, f.user, cfg); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.VerifyPOSTest(ctx, f.user, first.ID); !core.HasCode(e, "conflict") {
		t.Fatal("stale hardware test accepted", e)
	}
}
