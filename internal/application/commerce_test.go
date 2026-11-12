package application_test

import (
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/sales"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var ctx = context.Background()

type fixture struct {
	store *sqlite.Store
	app   *application.Service
	user  admin.User
	path  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "erp.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err = store.Migrate(ctx, nil); err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	// Commerce fixtures use repository ports; password hashing is exercised by auth tests.
	u := admin.User{ID: core.ID(), Username: "admin", Name: "Administradora", Role: "admin", Active: true, PasswordHash: "test-only", CreatedAt: core.Now()}
	err = store.WithinTx(ctx, func(p application.Ports) error {
		if e := p.Admin.CreateCompany(ctx, admin.Company{Name: "Empresa de prueba", Currency: "CLP", Timezone: "America/Santiago"}); e != nil {
			return e
		}
		return p.Admin.CreateUser(ctx, u)
	})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{store, app, u, path}
}
func (f *fixture) product(t *testing.T, sku, price, stock string) catalog.Product {
	t.Helper()
	p, err := f.app.CreateProduct(ctx, f.user, catalog.Product{SKU: sku, Name: "Producto " + sku, Unit: "kg", PriceMinor: price})
	if err != nil {
		t.Fatal(err)
	}
	if stock != "0" {
		_, err = f.app.ReceiveStock(ctx, f.user, inventory.Receipt{ProductID: p.ID, Quantity: stock, Reason: "Existencias iniciales"})
		if err != nil {
			t.Fatal(err)
		}
	}
	return p
}
func request(id, q string) sales.Request {
	return sales.Request{Lines: []sales.LineRequest{{ProductID: id, Quantity: q}}}
}
func (f *fixture) confirm(ctx context.Context, actor admin.User, key string, r sales.Request) (sales.Sale, bool, error) {
	q, e := f.app.QuoteSale(ctx, actor, r)
	if e != nil {
		return sales.Sale{}, false, e
	}
	r.QuoteFingerprint = q.Fingerprint
	return f.app.ConfirmSale(ctx, actor, key, r)
}
func count(t *testing.T, s *sqlite.Store, table string) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func stock(t *testing.T, f *fixture, id string) int64 {
	t.Helper()
	var q int64
	if err := f.store.DB.QueryRow("SELECT qty_micro/1000 FROM inventory_stock WHERE product_id=?", id).Scan(&q); err != nil {
		t.Fatal(err)
	}
	return q
}

func TestSaleExactAndDurableIdempotency(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "CAFE", "101", "10")
	key := core.Token()
	sale, replay, err := f.confirm(ctx, f.user, key, request(p.ID, "0.500"))
	if err != nil || replay || sale.TotalMinor != "51" {
		t.Fatalf("sale %+v replay=%v err=%v", sale, replay, err)
	}
	if stock(t, f, p.ID) != 9500 || count(t, f.store, "treasury_movements") != 1 {
		t.Fatal("inconsistent inventory/cash")
	}
	f.store.Close()
	reopened, err := sqlite.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	f.store = reopened
	f.app = application.New(reopened)
	again, replay, err := f.confirm(ctx, f.user, key, request(p.ID, "0.5"))
	if err != nil || !replay || again.ID != sale.ID || again.Number != sale.Number {
		t.Fatalf("persistent replay: %+v %v %v", again, replay, err)
	}
	if _, _, err = f.confirm(ctx, f.user, key, request(p.ID, "1")); !core.HasCode(err, "conflict") {
		t.Fatalf("different body: %v", err)
	}
	if count(t, f.store, "sales_documents") != 1 || stock(t, f, p.ID) != 9500 {
		t.Fatal("replay changed business state")
	}
}
func TestConcurrentSalesCannotOversell(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "LIMITADO", "1250", "3")
	var successes atomic.Int32
	var wg sync.WaitGroup
	durations := make(chan time.Duration, 10)
	start := time.Now()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			begin := time.Now()
			_, _, err := f.confirm(ctx, f.user, core.Token(), request(p.ID, "1"))
			durations <- time.Since(begin)
			if err == nil {
				successes.Add(1)
			} else if !core.HasCode(err, "conflict") {
				t.Errorf("unexpected sale error: %v", err)
			}
		}()
	}
	wg.Wait()
	close(durations)
	if successes.Load() != 3 || stock(t, f, p.ID) != 0 || count(t, f.store, "sales_documents") != 3 || count(t, f.store, "treasury_movements") != 3 {
		t.Fatalf("oversold: success=%d stock=%d", successes.Load(), stock(t, f, p.ID))
	}
	var audit int
	f.store.DB.QueryRow("SELECT count(*) FROM audit_entries WHERE action='sale.confirm'").Scan(&audit)
	if audit != 3 {
		t.Fatalf("audit=%d", audit)
	}
	var latencies []time.Duration
	for d := range durations {
		latencies = append(latencies, d)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	t.Logf("10 concurrent writers: elapsed=%s p95=%s", time.Since(start), latencies[9])
}
func TestConcurrentRetriesCreateOnlyOneSale(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "UNICO", "2000", "20")
	key := core.Token()
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sale, _, err := f.confirm(ctx, f.user, key, request(p.ID, "2"))
			if err != nil {
				t.Error(err)
				return
			}
			ids <- sale.ID
		}()
	}
	wg.Wait()
	close(ids)
	var first string
	for id := range ids {
		if first != "" && first != id {
			t.Fatal("different sales for one key")
		}
		first = id
	}
	if count(t, f.store, "sales_documents") != 1 || count(t, f.store, "treasury_movements") != 1 || stock(t, f, p.ID) != 18000 {
		t.Fatal("duplicate retry")
	}
}
func TestFailureRollsBackDocumentStockCashAndAudit(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "REVERSION", "100", "5")
	before := count(t, f.store, "audit_entries")
	if _, err := f.store.DB.Exec("CREATE TRIGGER reject_audit BEFORE INSERT ON audit_entries WHEN NEW.action='sale.confirm' BEGIN SELECT RAISE(ABORT,'injected failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.confirm(ctx, f.user, core.Token(), request(p.ID, "2")); err == nil {
		t.Fatal("injected failure ignored")
	}
	if stock(t, f, p.ID) != 5000 || count(t, f.store, "sales_documents") != 0 || count(t, f.store, "sales_lines") != 0 || count(t, f.store, "treasury_movements") != 0 || count(t, f.store, "inventory_movements") != 1 || count(t, f.store, "audit_entries") != before {
		t.Fatal("partial transaction committed")
	}
}
func TestInsufficientLineRollsBackOtherProducts(t *testing.T) {
	f := newFixture(t)
	a := f.product(t, "A", "100", "10")
	b := f.product(t, "B", "200", "0")
	_, _, err := f.confirm(ctx, f.user, core.Token(), sales.Request{Lines: []sales.LineRequest{{ProductID: a.ID, Quantity: "1"}, {ProductID: b.ID, Quantity: "1"}}})
	if !core.HasCode(err, "conflict") {
		t.Fatal(err)
	}
	if stock(t, f, a.ID) != 10000 || count(t, f.store, "sales_documents") != 0 {
		t.Fatal("partial inventory deduction")
	}
}
func TestOperatorCannotWriteCatalogOrStock(t *testing.T) {
	f := newFixture(t)
	operator := f.user
	operator.Role = "operator"
	if _, err := f.app.CreateProduct(ctx, operator, catalog.Product{}); !core.HasCode(err, "forbidden") {
		t.Fatal(err)
	}
	if _, err := f.app.ReceiveStock(ctx, operator, inventory.Receipt{}); !core.HasCode(err, "forbidden") {
		t.Fatal(err)
	}
}
func TestDuplicateCartLinesAreNormalized(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "PESO", "101", "2")
	key := core.Token()
	sale, _, err := f.confirm(ctx, f.user, key, sales.Request{Lines: []sales.LineRequest{{ProductID: p.ID, Quantity: "0.25"}, {ProductID: p.ID, Quantity: "0.250"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sale.Lines) != 1 || sale.TotalMinor != "51" {
		t.Fatalf("line rounding %+v", sale)
	}
	_, replay, err := f.confirm(ctx, f.user, key, request(p.ID, "0.5"))
	if err != nil || !replay {
		t.Fatalf("canonical replay %v %v", replay, err)
	}
}
func TestSearchEscapesWildcardsAndPagination(t *testing.T) {
	f := newFixture(t)
	for i, sku := range []string{"100%", "PLAIN", "A_B"} {
		f.product(t, fmt.Sprintf("%s-%d", sku, i), "100", "0")
	}
	page, err := f.store.Read().Reports.Products(ctx, core.Query{Search: "%"})
	if err != nil || page.Total != 1 {
		t.Fatalf("literal wildcard: %+v %v", page, err)
	}
	page, err = f.store.Read().Reports.Products(ctx, core.Query{Page: 2, PageSize: 2})
	if err != nil || page.Total != 3 || len(page.Items) != 1 {
		t.Fatalf("pagination %+v %v", page, err)
	}
}
