package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/sales"
	"business-manage/internal/modules/treasury"
	"business-manage/internal/pricing"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func normalizeSale(r sales.Request) (sales.Request, error) {
	if len(r.Lines) == 0 || len(r.Lines) > 100 {
		return r, core.Invalid("La venta debe tener entre 1 y 100 productos.")
	}
	quantities := map[string]int64{}
	for _, line := range r.Lines {
		if len(line.ProductID) != 32 {
			return r, core.Invalid("Producto inválido.")
		}
		q, e := core.Quantity(line.Quantity)
		if e != nil {
			return r, e
		}
		if q <= 0 {
			return r, core.Invalid("La cantidad debe ser positiva.")
		}
		quantities[line.ProductID] += q
		if quantities[line.ProductID] > 1_000_000_000_000 {
			return r, core.Invalid("Cantidad fuera de rango.")
		}
	}
	r.Lines = []sales.LineRequest{}
	for id, q := range quantities {
		r.Lines = append(r.Lines, sales.LineRequest{ProductID: id, Quantity: core.QuantityText(q)})
	}
	sort.Slice(r.Lines, func(i, j int) bool { return r.Lines[i].ProductID < r.Lines[j].ProductID })
	if len(r.LineDiscounts) > 100 || len(r.Rewards) > 100 {
		return r, core.Invalid("Demasiados descuentos o regalos.")
	}
	r.LineDiscounts = append([]sales.LineDiscount(nil), r.LineDiscounts...)
	seen := map[string]bool{}
	for i := range r.LineDiscounts {
		d := &r.LineDiscounts[i]
		if quantities[d.ProductID] == 0 || seen[d.ProductID] {
			return r, core.Invalid("Descuento repetido o sin producto de venta.")
		}
		seen[d.ProductID] = true
		if e := d.Discount.Normalize(); e != nil {
			return r, e
		}
	}
	sort.Slice(r.LineDiscounts, func(i, j int) bool { return r.LineDiscounts[i].ProductID < r.LineDiscounts[j].ProductID })
	if r.TotalDiscount != nil {
		d := *r.TotalDiscount
		r.TotalDiscount = &d
		if e := r.TotalDiscount.Normalize(); e != nil {
			return r, e
		}
		if r.TotalDiscount.Basis == "unit" {
			return r, core.Invalid("El descuento total no se aplica por unidad.")
		}
	}
	r.DiscountReason = strings.TrimSpace(r.DiscountReason)
	if len(r.LineDiscounts) > 0 || r.TotalDiscount != nil {
		if len(r.DiscountReason) < 3 || len(r.DiscountReason) > 240 {
			return r, core.Invalid("Indica un motivo de descuento de 3 a 240 caracteres.")
		}
	} else {
		r.DiscountReason = ""
	}
	grouped := map[string]sales.RewardSelection{}
	for _, reward := range r.Rewards {
		if len(reward.ProductID) != 32 || len(reward.PromotionID) != 32 {
			return r, core.Invalid("Regalo inválido.")
		}
		q, e := core.Quantity(reward.Quantity)
		if e != nil || q <= 0 {
			return r, core.Invalid("La cantidad de regalo debe ser positiva.")
		}
		key := reward.PromotionID + ":" + reward.ProductID
		prior := grouped[key]
		if prior.Quantity != "" {
			old, _ := core.Quantity(prior.Quantity)
			q += old
		}
		if q > 1_000_000_000_000 {
			return r, core.Invalid("Cantidad de regalo fuera de rango.")
		}
		reward.Quantity = core.QuantityText(q)
		grouped[key] = reward
	}
	r.Rewards = nil
	for _, reward := range grouped {
		r.Rewards = append(r.Rewards, reward)
	}
	sort.Slice(r.Rewards, func(i, j int) bool {
		a, b := r.Rewards[i], r.Rewards[j]
		return a.PromotionID+":"+a.ProductID < b.PromotionID+":"+b.ProductID
	})
	return r, nil
}
func quoteData(ctx context.Context, p Ports, r sales.Request, actor admin.User, at time.Time) (sales.Quote, error) {
	rules, e := p.Promotions.All(ctx)
	if e != nil {
		return sales.Quote{}, e
	}
	base := map[string]bool{}
	needed := map[string]bool{}
	for _, l := range r.Lines {
		base[l.ProductID] = true
		needed[l.ProductID] = true
	}
	relevant := []promotions.Rule{}
	for _, rule := range rules {
		if !rule.Active(at) {
			continue
		}
		match := rule.Scope == "cart"
		for _, id := range rule.ProductIDs {
			if base[id] {
				match = true
			}
		}
		if !match {
			continue
		}
		relevant = append(relevant, rule)
		for _, id := range rule.RewardProductIDs {
			needed[id] = true
		}
	}
	for _, gift := range r.Rewards {
		needed[gift.ProductID] = true
	}
	products := map[string]catalog.Product{}
	for id := range needed {
		product, e := p.Catalog.Product(ctx, id)
		if e != nil {
			return sales.Quote{}, e
		}
		products[id] = product
	}
	inputs := map[string]bool{}
	for _, product := range products {
		for _, component := range product.Components {
			inputs[component.ProductID] = true
		}
	}
	for id := range inputs {
		if _, ok := products[id]; ok {
			continue
		}
		input, e := p.Catalog.Product(ctx, id)
		if e != nil {
			return sales.Quote{}, e
		}
		products[id] = input
	}
	balances := map[string]int64{}
	for id := range products {
		q, e := p.Inventory.Stock(ctx, id)
		if e != nil {
			return sales.Quote{}, e
		}
		balances[id] = q
	}
	return pricing.Calculate(r, products, balances, relevant, actor, at)
}
func (s *Service) QuoteSale(ctx context.Context, actor admin.User, r sales.Request) (sales.Quote, error) {
	var result sales.Quote
	r, e := normalizeSale(r)
	if e != nil {
		return result, e
	}
	e = s.Store.ReadSnapshot(ctx, func(p Ports) error {
		current, e := p.Admin.User(ctx, actor.ID)
		if e != nil {
			return e
		}
		if !current.Active {
			return core.Unauthorized
		}
		result, e = quoteData(ctx, p, r, current, s.Now())
		return e
	})
	return result, e
}
func (s *Service) ConfirmSale(ctx context.Context, actor admin.User, key string, r sales.Request) (sales.Sale, bool, error) {
	if actor.Role != "admin" && actor.Role != "operator" {
		return sales.Sale{}, false, core.Forbidden
	}
	if len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key {
		return sales.Sale{}, false, core.Invalid("La clave de idempotencia debe tener entre 16 y 128 caracteres.")
	}
	r, e := normalizeSale(r)
	if e != nil {
		return sales.Sale{}, false, e
	}
	fingerprint := r.QuoteFingerprint
	r.QuoteFingerprint = ""
	canonical, _ := json.Marshal(r)
	hash := digest(actor.ID + ":" + string(canonical))
	var result sales.Sale
	replayed := false
	e = s.Store.WithinTx(ctx, func(p Ports) error {
		prior, e := p.Sales.SaleByKey(ctx, key)
		if e == nil {
			if prior.RequestHash != hash {
				return core.Conflict("La clave de idempotencia ya corresponde a otra venta.")
			}
			result = prior
			replayed = true
			return nil
		}
		if !core.HasCode(e, "not_found") {
			return e
		}
		current, e := p.Admin.User(ctx, actor.ID)
		if e != nil {
			return e
		}
		if !current.Active {
			return core.Unauthorized
		}
		quote, e := quoteData(ctx, p, r, current, s.Now())
		if e != nil {
			return e
		}
		if fingerprint == "" || fingerprint != quote.Fingerprint {
			return core.Conflict("La cotización cambió o debe actualizarse. Revisa el total y confirma nuevamente.")
		}
		ids := map[string]string{}
		for i := range quote.Lines {
			old := quote.Lines[i].ID
			quote.Lines[i].ID = core.ID()
			ids[old] = quote.Lines[i].ID
		}
		for i := range quote.Discounts {
			if id, ok := ids[quote.Discounts[i].LineID]; ok {
				quote.Discounts[i].LineID = id
			}
		}
		result = sales.Sale{Result: quote.Result, ID: core.ID(), CreatedAt: core.Now(), ActorID: actor.ID, IdempotencyKey: key, RequestHash: hash}
		result.Company, e = p.Admin.Company(ctx)
		if e != nil {
			return e
		}
		printer, e := p.POS.Settings(ctx)
		if e != nil {
			return e
		}
		if printer.Enabled {
			result.PrintJobID = core.ID()
		}
		if e = p.Sales.CreateSale(ctx, &result); e != nil {
			return e
		}
		demand := map[string]int64{}
		policies := map[string]string{}
		for _, l := range result.Lines {
			for _, c := range l.Consumptions {
				q, _ := core.StockQuantity(c.Quantity)
				demand[c.ProductID] += q
				policies[c.ProductID] = c.StockPolicy
			}
		}
		resources := []string{}
		for id := range demand {
			resources = append(resources, id)
		}
		sort.Strings(resources)
		for _, id := range resources {
			if e = p.Inventory.Consume(ctx, id, demand[id], policies[id], result.ID, actor.ID, result.CreatedAt); e != nil {
				return e
			}
		}
		if e = p.Treasury.CreditSale(ctx, treasury.Movement{ID: core.ID(), SaleID: result.ID, SaleNumber: result.Number, AmountMinor: result.TotalMinor, CreatedAt: result.CreatedAt}, actor.ID); e != nil {
			return e
		}
		if len(r.LineDiscounts) > 0 || r.TotalDiscount != nil {
			entry := auditEntry(actor.ID, "sale.manual_discount", result.ID)
			entry.Details = map[string]any{"discounts": result.Discounts, "reason": r.DiscountReason, "amountMinor": result.ManualDiscountMinor}
			if e = p.Audit.Append(ctx, entry); e != nil {
				return e
			}
		}
		if e = p.Audit.Append(ctx, auditEntry(actor.ID, "sale.confirm", result.ID)); e != nil {
			return e
		}
		if result.PrintJobID != "" {
			payload := pos.Receipt(result, printer, false)
			if printer.DrawerEnabled {
				payload = append(payload, pos.Drawer(printer)...)
			}
			job := newPrintJob(actor.ID, "sale", "sale:"+result.ID, digest(result.ID), printer, payload, printer.DrawerEnabled, "")
			job.ID = result.PrintJobID
			job.SaleID = result.ID
			job.SaleNumber = result.Number
			if e = p.POS.Enqueue(ctx, job); e != nil {
				return e
			}
			return logPrintJob(ctx, p, job, "pos.enqueue")
		}
		return nil
	})
	return result, replayed, e
}
