package application_test

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/sales"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func micro(t *testing.T, f *fixture, id string) int64 {
	t.Helper()
	q, e := f.store.Read().Inventory.Stock(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func composed(t *testing.T, f *fixture, name, price string, inputs []catalog.Component) catalog.Product {
	t.Helper()
	p, e := f.app.CreateProduct(ctx, f.user, catalog.Product{Name: name, SKU: name, Unit: "un", PriceMinor: price, Sellable: true, StockMode: "components", StockPolicy: "block", Components: inputs})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSharedInputsAndFractionalConsumption(t *testing.T) {
	f := newFixture(t)
	paper := f.product(t, "PAPEL", "0", "10")
	color := composed(t, f, "Color", "200", []catalog.Component{{ProductID: paper.ID, Quantity: "1"}})
	bw := composed(t, f, "Blanco y negro", "50", []catalog.Component{{ProductID: paper.ID, Quantity: "1"}})
	sale, _, e := f.confirm(ctx, f.user, core.Token(), sales.Request{Lines: []sales.LineRequest{{ProductID: color.ID, Quantity: "3"}, {ProductID: bw.ID, Quantity: "4"}}})
	if e != nil {
		t.Fatal(e)
	}
	if sale.TotalMinor != "800" || micro(t, f, paper.ID) != 3e6 {
		t.Fatalf("shared input %+v", sale)
	}
	var movements int
	f.store.DB.QueryRow("SELECT count(*) FROM inventory_movements WHERE sale_id=?", sale.ID).Scan(&movements)
	if movements != 1 {
		t.Fatal("input deducted more than once")
	}
	partial := composed(t, f, "Fraccionado", "100", []catalog.Component{{ProductID: paper.ID, Quantity: "0.125"}})
	sale, _, e = f.confirm(ctx, f.user, core.Token(), request(partial.ID, "0.25"))
	if e != nil {
		t.Fatal(e)
	}
	if micro(t, f, paper.ID) != 2_968_750 || sale.Lines[0].Consumptions[0].Quantity != "0.031250" {
		t.Fatalf("rounded consumption %+v", sale)
	}
}
func TestStockPoliciesAndReactivation(t *testing.T) {
	for _, policy := range []string{"block", "negative", "zero", "untracked"} {
		t.Run(policy, func(t *testing.T) {
			f := newFixture(t)
			p := f.product(t, "POLICY", "100", "1")
			p.StockPolicy = policy
			p, e := f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p})
			if e != nil {
				t.Fatal(e)
			}
			_, _, e = f.confirm(ctx, f.user, core.Token(), request(p.ID, "2"))
			if policy == "block" {
				if !core.HasCode(e, "conflict") || micro(t, f, p.ID) != 1e6 {
					t.Fatalf("block: %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			want := int64(0)
			if policy == "negative" {
				want = -1e6
			}
			if policy == "untracked" {
				want = 1e6
			}
			if micro(t, f, p.ID) != want {
				t.Fatal("wrong policy balance")
			}
			var sum int64
			f.store.DB.QueryRow("SELECT sum(qty_micro) FROM inventory_movements WHERE product_id=?", p.ID).Scan(&sum)
			if sum != want {
				t.Fatalf("ledger=%d stock=%d", sum, want)
			}
			p.StockPolicy = "block"
			if policy == "untracked" {
				if _, e = f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p}); e == nil {
					t.Fatal("reactivated without count")
				}
				count := "5"
				if _, e = f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p, CountedStock: &count, AdjustmentReason: "Conteo de reactivación"}); e != nil {
					t.Fatal(e)
				}
				if micro(t, f, p.ID) != 5e6 {
					t.Fatal("wrong counted balance")
				}
			}
			if policy == "negative" {
				p.StockPolicy = "zero"
				if _, e = f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p}); e != nil {
					t.Fatal(e)
				}
				if micro(t, f, p.ID) != 0 {
					t.Fatal("negative balance not reset")
				}
				var resets int
				if e = f.store.DB.QueryRow("SELECT count(*) FROM audit_entries WHERE action='product.update' AND entity_id=? AND json_extract(details_json,'$.stockBefore')='-1.000000' AND json_extract(details_json,'$.stockAfter')='0.000000'", p.ID).Scan(&resets); e != nil || resets != 1 {
					t.Fatal("stock reset was not audited", e, resets)
				}
			}
		})
	}
}
func TestConcurrentServicesRespectOneSharedStock(t *testing.T) {
	f := newFixture(t)
	input := f.product(t, "LIMITED", "0", "5")
	a := composed(t, f, "Servicio A", "100", []catalog.Component{{ProductID: input.ID, Quantity: "1"}})
	b := composed(t, f, "Servicio B", "200", []catalog.Component{{ProductID: input.ID, Quantity: "1"}})
	var good atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		p := a
		if i%2 == 0 {
			p = b
		}
		go func(p catalog.Product) {
			defer wg.Done()
			_, _, e := f.confirm(ctx, f.user, core.Token(), request(p.ID, "1"))
			if e == nil {
				good.Add(1)
			} else if !core.HasCode(e, "conflict") {
				t.Error(e)
			}
		}(p)
	}
	wg.Wait()
	if good.Load() != 5 || micro(t, f, input.ID) != 0 {
		t.Fatalf("oversold shared input: %d", good.Load())
	}
}
func TestQuotesHistoryPermissionsAndExpiredRetry(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "PROMO", "100", "20")
	at := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	f.app.Now = func() time.Time { return at }
	end := at.Add(time.Hour).Format(time.RFC3339)
	rule, e := f.app.SavePromotion(ctx, f.user, "", promotions.Rule{Name: "Descuento veinte", Kind: "discount", Scope: "products", ProductIDs: []string{p.ID}, Discount: &core.Discount{Type: "percentage", Value: "20"}, Enabled: true, Priority: 100, EndsAt: &end})
	if e != nil {
		t.Fatal(e)
	}
	r := request(p.ID, "1")
	r.LineDiscounts = []sales.LineDiscount{{ProductID: p.ID, Discount: core.Discount{Type: "percentage", Value: "10"}}}
	r.TotalDiscount = &core.Discount{Type: "percentage", Value: "50"}
	r.DiscountReason = "Cliente habitual"
	quote, e := f.app.QuoteSale(ctx, f.user, r)
	if e != nil {
		t.Fatal(e)
	}
	r.QuoteFingerprint = quote.Fingerprint
	key := core.Token()
	sale, _, e := f.app.ConfirmSale(ctx, f.user, key, r)
	if e != nil {
		t.Fatal(e)
	}
	if sale.TotalMinor != "36" {
		t.Fatal(sale.TotalMinor)
	}
	at = at.Add(2 * time.Hour)
	again, replay, e := f.app.ConfirmSale(ctx, f.user, key, r)
	if e != nil || !replay || again.TotalMinor != "36" {
		t.Fatalf("expired retry %+v %v", again, e)
	}
	if _, _, e = f.app.ConfirmSale(ctx, f.user, core.Token(), r); !core.HasCode(e, "conflict") {
		t.Fatal("stale quote accepted")
	}
	_ = rule
	p.Name = "Nombre modificado"
	p.PriceMinor = "500"
	if _, e = f.app.UpdateProduct(ctx, f.user, p.ID, application.ProductUpdate{Product: p}); e != nil {
		t.Fatal(e)
	}
	saved, e := f.store.Read().Sales.Sale(ctx, sale.ID)
	if e != nil || saved.Lines[0].Name == p.Name || saved.Lines[0].PriceMinor != "100" {
		t.Fatal("history changed", e)
	}
	if _, e = f.app.UpdateCompany(ctx, f.user, "Empresa renombrada", "UTC"); e != nil {
		t.Fatal(e)
	}
	saved, _ = f.store.Read().Sales.Sale(ctx, sale.ID)
	if saved.Company.Name == "Empresa renombrada" {
		t.Fatal("historical company changed")
	}
}
func TestDiscountPermissionIsAuditedAndRevokedBeforeConfirm(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "PERMISSION", "100", "3")
	operator := admin.User{ID: core.ID(), Username: "operator", Name: "Operador", Role: "operator", Active: true, PasswordHash: "fixture", CreatedAt: core.Now()}
	if e := f.store.WithinTx(ctx, func(p application.Ports) error { return p.Admin.CreateUser(ctx, operator) }); e != nil {
		t.Fatal(e)
	}
	r := request(p.ID, "1")
	r.TotalDiscount = &core.Discount{Type: "amount", Value: "10", Basis: "line"}
	r.DiscountReason = "Autorización comercial"
	if _, e := f.app.QuoteSale(ctx, operator, r); !core.HasCode(e, "forbidden") {
		t.Fatal(e)
	}
	if e := f.app.SetDiscountPermission(ctx, f.user, operator.ID, true); e != nil {
		t.Fatal(e)
	}
	q, e := f.app.QuoteSale(ctx, operator, r)
	if e != nil {
		t.Fatal(e)
	}
	r.QuoteFingerprint = q.Fingerprint
	if e = f.app.SetDiscountPermission(ctx, f.user, operator.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.app.ConfirmSale(ctx, operator, core.Token(), r); !core.HasCode(e, "forbidden") {
		t.Fatal("revoked permission accepted", e)
	}
	var logs int
	f.store.DB.QueryRow("SELECT count(*) FROM audit_entries WHERE action='user.discount_permission'").Scan(&logs)
	if logs != 2 {
		t.Fatal(logs)
	}
}
func TestCompositionAndUnitEditsCannotCorruptHistory(t *testing.T) {
	f := newFixture(t)
	a := f.product(t, "A", "100", "2")
	b := composed(t, f, "Producto B", "100", []catalog.Component{{ProductID: a.ID, Quantity: "1"}})
	if _, e := f.app.CreateProduct(context.Background(), f.user, catalog.Product{SKU: "NESTED", Name: "Anidado", Unit: "un", PriceMinor: "100", Sellable: true, StockMode: "components", Components: []catalog.Component{{ProductID: b.ID, Quantity: "1"}}}); e == nil {
		t.Fatal("nested composition accepted")
	}
	a.Unit = "lt"
	if _, e := f.app.UpdateProduct(ctx, f.user, a.ID, application.ProductUpdate{Product: a}); e == nil {
		t.Fatal("historical unit changed")
	}
}
func TestGiftConsumesInputsAndIndependentCountIsAudited(t *testing.T) {
	f := newFixture(t)
	paper := f.product(t, "GIFT-PAPER", "0", "20")
	copy := composed(t, f, "Fotocopia", "200", []catalog.Component{{ProductID: paper.ID, Quantity: "1"}})
	rule, e := f.app.SavePromotion(ctx, f.user, "", promotions.Rule{Name: "Cada diez una más", Kind: "buy_get", Scope: "products", ProductIDs: []string{copy.ID}, RewardProductIDs: []string{copy.ID}, BuyQuantity: "10", GiftQuantity: "1", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	r := request(copy.ID, "10")
	r.Rewards = []sales.RewardSelection{{PromotionID: rule.ID, ProductID: copy.ID, Quantity: "1"}}
	sale, _, e := f.confirm(ctx, f.user, core.Token(), r)
	if e != nil || sale.TotalMinor != "2000" || micro(t, f, paper.ID) != 9e6 {
		t.Fatalf("gift %+v %v", sale, e)
	}
	if _, e = f.app.AdjustStock(ctx, f.user, inventory.Adjustment{ProductID: paper.ID, CountedQuantity: "8.500001", Reason: "Conteo físico"}); e != nil {
		t.Fatal(e)
	}
	if micro(t, f, paper.ID) != 8_500_001 {
		t.Fatal("count precision lost")
	}
}
