package pricing_test

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/sales"
	"business-manage/internal/pricing"
	"testing"
	"time"
)

func product(id, price string) catalog.Product {
	return catalog.Product{ID: id, Name: "Producto " + id, SKU: id, Unit: "un", PriceMinor: price, Sellable: true, StockMode: "own", StockPolicy: "block", Components: []catalog.Component{}, Version: 1}
}
func TestPriorityManualStackingAndExactAllocation(t *testing.T) {
	p := product(core.ID(), "100")
	percent := func(value string) *core.Discount {
		return &core.Discount{Type: "percentage", Value: value, Basis: "line"}
	}
	r := sales.Request{Lines: []sales.LineRequest{{ProductID: p.ID, Quantity: "1"}}, LineDiscounts: []sales.LineDiscount{{ProductID: p.ID, Discount: *percent("10")}}, TotalDiscount: percent("50"), DiscountReason: "Cliente habitual"}
	rules := []promotions.Rule{{ID: core.ID(), Name: "Veinte", Kind: "discount", Scope: "products", ProductIDs: []string{p.ID}, Enabled: true, Priority: 200, Discount: percent("20")}, {ID: core.ID(), Name: "Diez", Kind: "discount", Scope: "products", ProductIDs: []string{p.ID}, Enabled: true, Stackable: true, Priority: 100, Discount: percent("10")}}
	q, e := pricing.Calculate(r, map[string]catalog.Product{p.ID: p}, map[string]int64{p.ID: 1e6}, rules, admin.User{Role: "admin"}, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if q.TotalMinor != "36" || q.PromotionDiscountMinor != "20" || q.ManualDiscountMinor != "44" {
		t.Fatalf("totals %+v", q.Result)
	}
	got := pricing.Allocate(2, []int64{1, 1, 1}, []string{"a", "b", "c"})
	if got[0] != 1 || got[1] != 1 || got[2] != 0 {
		t.Fatal(got)
	}
}
func TestBuyPayDiscountsCheapestWithoutUnitExpansion(t *testing.T) {
	a, b, c := product(core.ID(), "100"), product(core.ID(), "80"), product(core.ID(), "50")
	products := map[string]catalog.Product{a.ID: a, b.ID: b, c.ID: c}
	r := sales.Request{Lines: []sales.LineRequest{{ProductID: a.ID, Quantity: "1"}, {ProductID: b.ID, Quantity: "1"}, {ProductID: c.ID, Quantity: "1"}}}
	rule := promotions.Rule{ID: core.ID(), Name: "Tres por dos", Kind: "buy_pay", Scope: "products", ProductIDs: []string{a.ID, b.ID, c.ID}, BuyQuantity: "3", PayQuantity: "2", Enabled: true}
	q, e := pricing.Calculate(r, products, map[string]int64{}, []promotions.Rule{rule}, admin.User{Role: "admin"}, time.Now())
	if e != nil || q.TotalMinor != "180" {
		t.Fatalf("%+v %v", q.Result, e)
	}
	a.PriceMinor = "1"
	products[a.ID] = a
	r.Lines = []sales.LineRequest{{ProductID: a.ID, Quantity: "1000000000"}}
	rule.BuyQuantity = "100"
	rule.PayQuantity = "50"
	q, e = pricing.Calculate(r, products, map[string]int64{}, []promotions.Rule{rule}, admin.User{Role: "admin"}, time.Now())
	if e != nil || q.TotalMinor != "500000000" || len(q.Lines) != 1 {
		t.Fatalf("large quantity %+v %v", q.Result, e)
	}
}
func TestGiftIsAdditionalAndDoesNotGenerateAnotherGift(t *testing.T) {
	p := product(core.ID(), "100")
	rule := promotions.Rule{ID: core.ID(), Name: "Diez más uno", Kind: "buy_get", Scope: "products", ProductIDs: []string{p.ID}, RewardProductIDs: []string{p.ID}, BuyQuantity: "10", GiftQuantity: "1", Enabled: true, Stackable: true}
	r := sales.Request{Lines: []sales.LineRequest{{ProductID: p.ID, Quantity: "10"}}, Rewards: []sales.RewardSelection{{PromotionID: rule.ID, ProductID: p.ID, Quantity: "1"}}}
	q, e := pricing.Calculate(r, map[string]catalog.Product{p.ID: p}, map[string]int64{p.ID: 20e6}, []promotions.Rule{rule}, admin.User{Role: "admin"}, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Lines) != 2 || q.TotalMinor != "1000" || q.SubtotalMinor != "1100" || q.RewardOptions[0].MaximumQuantity != "1.000" || q.Availability[0].RequiredQuantity != "11.000000" {
		t.Fatalf("gift %+v", q)
	}
}
func TestValidityBoundariesAndExcessManualDiscount(t *testing.T) {
	p := product(core.ID(), "100")
	start, end := "2026-10-30T10:00:00Z", "2026-10-30T11:00:00Z"
	rule := promotions.Rule{Enabled: true, StartsAt: &start, EndsAt: &end}
	for _, test := range []struct {
		at     string
		active bool
	}{{"2026-10-30T09:59:59Z", false}, {start, true}, {end, false}} {
		at, _ := time.Parse(time.RFC3339, test.at)
		if rule.Active(at) != test.active {
			t.Fatal(test)
		}
	}
	r := sales.Request{Lines: []sales.LineRequest{{ProductID: p.ID, Quantity: "1"}}, TotalDiscount: &core.Discount{Type: "amount", Value: "101", Basis: "line"}, DiscountReason: "Descuento especial"}
	if _, e := pricing.Calculate(r, map[string]catalog.Product{p.ID: p}, map[string]int64{}, nil, admin.User{Role: "admin"}, time.Now()); e == nil {
		t.Fatal("negative total accepted")
	}
}

func TestFixedOffersCapLargeAmountsAndCartDiscountsReconcile(t *testing.T) {
	a, b := product("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1"), product("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "2")
	rule := promotions.Rule{ID: core.ID(), Name: "Monto por unidad", Kind: "discount", Scope: "products", ProductIDs: []string{a.ID}, Enabled: true, Discount: &core.Discount{Type: "amount", Value: "1000000000000000", Basis: "unit"}}
	request := sales.Request{Lines: []sales.LineRequest{{ProductID: a.ID, Quantity: "1000000000"}}}
	q, e := pricing.Calculate(request, map[string]catalog.Product{a.ID: a}, nil, []promotions.Rule{rule}, admin.User{Role: "admin"}, time.Now())
	if e != nil || q.TotalMinor != "0" || q.PromotionDiscountMinor != "1000000000" {
		t.Fatalf("large automatic amount %+v %v", q.Result, e)
	}
	request = sales.Request{Lines: []sales.LineRequest{{ProductID: a.ID, Quantity: "1"}, {ProductID: b.ID, Quantity: "1"}}, TotalDiscount: &core.Discount{Type: "amount", Value: "1", Basis: "line"}, DiscountReason: "Descuento total"}
	rule.Scope = "cart"
	rule.Discount = &core.Discount{Type: "percentage", Value: "33.33", Basis: "line"}
	q, e = pricing.Calculate(request, map[string]catalog.Product{a.ID: a, b.ID: b}, nil, []promotions.Rule{rule}, admin.User{Role: "admin"}, time.Now())
	if e != nil || q.TotalMinor != "1" || q.PromotionDiscountMinor != "1" || q.ManualDiscountMinor != "1" {
		t.Fatalf("cart allocation %+v %v", q.Result, e)
	}
	var sum int64
	for _, l := range q.Lines {
		gross, _ := core.Minor(l.GrossMinor)
		auto, _ := core.Minor(l.PromotionDiscountMinor)
		manual, _ := core.Minor(l.OrderDiscountMinor)
		net, _ := core.Minor(l.TotalMinor)
		if gross-auto-manual != net || net < 0 {
			t.Fatal(l)
		}
		sum += net
	}
	if sum != 1 {
		t.Fatal("line totals do not reconcile")
	}
}
