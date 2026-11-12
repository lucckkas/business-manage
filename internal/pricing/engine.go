// Package pricing calculates immutable quotes using exact arithmetic and aggregate
// quantities. It has no persistence, transport, clock, or per-unit expansion.
package pricing

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/sales"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
	"time"
)

type priced struct {
	line                                         sales.Line
	qty, price, gross, left, auto, manual, order int64
	used, exclusive                              bool
}

func text(n int64) string { return strconv.FormatInt(n, 10) }
func includes(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
func appendLine(p catalog.Product, q int64, kind, rule string) (*priced, error) {
	if !p.Sellable {
		return nil, core.Conflict("El producto no está disponible para la venta: " + p.Name)
	}
	price, e := core.Minor(p.PriceMinor)
	if e != nil {
		return nil, e
	}
	gross, e := core.LineTotal(price, q)
	if e != nil {
		return nil, e
	}
	id := kind + ":" + p.ID
	if rule != "" {
		id += ":" + rule
	}
	return &priced{line: sales.Line{ID: id, Kind: kind, PromotionID: rule, ProductID: p.ID, SKU: p.SKU, Name: p.Name, Unit: p.Unit, Quantity: core.QuantityText(q), PriceMinor: p.PriceMinor, Consumptions: []sales.Consumption{}}, qty: q, price: price, gross: gross, left: gross}, nil
}
func eligible(lines []*priced, r promotions.Rule) []*priced {
	out := []*priced{}
	for _, l := range lines {
		if l.line.Kind == "regular" && (r.Scope == "cart" || includes(r.ProductIDs, l.line.ProductID)) {
			out = append(out, l)
		}
	}
	return out
}
func compatible(lines []*priced, r promotions.Rule) bool {
	for _, l := range lines {
		if l.used && (l.exclusive || !r.Stackable) {
			return false
		}
	}
	return true
}
func reserve(lines []*priced, r promotions.Rule) {
	for _, l := range lines {
		l.used = true
		l.exclusive = l.exclusive || !r.Stackable
	}
}

// Allocate distributes minor units with the largest-remainder method. At most
// len(lines)-1 remaining units are distributed, regardless of the amount.
func Allocate(amount int64, weights []int64, keys []string) []int64 {
	out := make([]int64, len(weights))
	var sum int64
	for _, w := range weights {
		sum += w
	}
	if sum == 0 || amount == 0 {
		return out
	}
	type remainder struct {
		index int
		value *big.Int
	}
	remainders := make([]remainder, len(weights))
	remaining := amount
	for i, w := range weights {
		n := new(big.Int).Mul(big.NewInt(amount), big.NewInt(w))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(n, big.NewInt(sum), r)
		out[i] = q.Int64()
		remaining -= out[i]
		remainders[i] = remainder{i, r}
	}
	sort.Slice(remainders, func(i, j int) bool {
		cmp := remainders[i].value.Cmp(remainders[j].value)
		if cmp == 0 {
			return keys[remainders[i].index] < keys[remainders[j].index]
		}
		return cmp > 0
	})
	for i := int64(0); i < remaining; i++ {
		out[remainders[i].index]++
	}
	return out
}
func distribute(amount int64, lines []*priced, kind string) {
	weights := make([]int64, len(lines))
	keys := make([]string, len(lines))
	for i, l := range lines {
		weights[i] = l.left
		keys[i] = l.line.ID
	}
	for i, n := range Allocate(amount, weights, keys) {
		lines[i].left -= n
		switch kind {
		case "auto":
			lines[i].auto += n
		case "order":
			lines[i].order += n
		}
	}
}
func Calculate(req sales.Request, products map[string]catalog.Product, balances map[string]int64, rules []promotions.Rule, actor admin.User, at time.Time) (sales.Quote, error) {
	quote := sales.Quote{Result: sales.Result{Lines: []sales.Line{}, Discounts: []sales.AppliedDiscount{}}, RewardOptions: []sales.RewardOption{}, Availability: []sales.Availability{}}
	lines := []*priced{}
	for _, request := range req.Lines {
		p, ok := products[request.ProductID]
		if !ok {
			return quote, core.NotFound
		}
		q, e := core.Quantity(request.Quantity)
		if e != nil || q <= 0 {
			return quote, core.Invalid("Cantidad inválida.")
		}
		l, e := appendLine(p, q, "regular", "")
		if e != nil {
			return quote, e
		}
		lines = append(lines, l)
	}
	active := []promotions.Rule{}
	for _, r := range rules {
		if r.Active(at) {
			active = append(active, r)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].Priority != active[j].Priority {
			return active[i].Priority > active[j].Priority
		}
		return active[i].ID < active[j].ID
	})
	selections := map[string][]sales.RewardSelection{}
	for _, s := range req.Rewards {
		selections[s.PromotionID] = append(selections[s.PromotionID], s)
	}
	for _, r := range active {
		matched := eligible(lines, r)
		if len(matched) == 0 || !compatible(matched, r) {
			continue
		}
		var quantity, total int64
		for _, l := range matched {
			quantity += l.qty
			total += l.left
		}
		if total > 1_000_000_000_000_000 {
			return quote, core.Invalid("El importe de la venta excede el máximo.")
		}
		applied := int64(0)
		switch r.Kind {
		case "discount":
			if r.Discount == nil {
				return quote, core.Invalid("Oferta incompleta.")
			}
			if r.Scope == "cart" {
				n, e := r.Discount.CappedAmount(total, 1000)
				if e != nil {
					return quote, e
				}
				if n > total {
					n = total
				}
				distribute(n, matched, "auto")
				applied = n
				if n > 0 {
					snapshot := r
					quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "promotion", Label: r.Name, AmountMinor: text(n), Promotion: &snapshot})
				}
			} else {
				for _, l := range matched {
					n, e := r.Discount.CappedAmount(l.left, l.qty)
					if e != nil {
						return quote, e
					}
					if n > l.left {
						n = l.left
					}
					l.left -= n
					l.auto += n
					applied += n
					if n > 0 {
						snapshot := r
						quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "promotion", LineID: l.line.ID, Label: r.Name, AmountMinor: text(n), Promotion: &snapshot})
					}
				}
			}
		case "buy_pay":
			buy, _ := core.Quantity(r.BuyQuantity)
			pay, _ := core.Quantity(r.PayQuantity)
			if buy <= 0 || pay <= 0 || pay >= buy {
				return quote, core.Invalid("Oferta por cantidad inválida.")
			}
			free := (quantity / buy) * (buy - pay)
			cheap := append([]*priced{}, matched...)
			sort.Slice(cheap, func(i, j int) bool {
				if cheap[i].price != cheap[j].price {
					return cheap[i].price < cheap[j].price
				}
				return cheap[i].line.ID < cheap[j].line.ID
			})
			for _, l := range cheap {
				q := min(free, l.qty)
				if q == 0 {
					continue
				}
				n, e := core.LineTotal(l.price, q)
				if e != nil {
					return quote, e
				}
				n = min(n, l.left)
				l.left -= n
				l.auto += n
				applied += n
				free -= q
				if n > 0 {
					snapshot := r
					quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "promotion", LineID: l.line.ID, Label: r.Name, AmountMinor: text(n), Promotion: &snapshot})
				}
			}
		case "buy_get":
			buy, _ := core.Quantity(r.BuyQuantity)
			gift, _ := core.Quantity(r.GiftQuantity)
			if buy <= 0 || gift <= 0 {
				return quote, core.Invalid("Oferta de regalo inválida.")
			}
			capacity := new(big.Int).Mul(big.NewInt(quantity/buy), big.NewInt(gift))
			if capacity.Sign() == 0 {
				continue
			}
			maxGift := int64(1_000_000_000_000)
			if capacity.Cmp(big.NewInt(maxGift)) < 0 {
				maxGift = capacity.Int64()
			}
			option := sales.RewardOption{PromotionID: r.ID, Name: r.Name, MaximumQuantity: core.QuantityText(maxGift), Products: []sales.RewardProduct{}}
			for _, id := range r.RewardProductIDs {
				p, ok := products[id]
				if ok && p.Sellable {
					option.Products = append(option.Products, sales.RewardProduct{ProductID: id, Name: p.Name, SKU: p.SKU, Unit: p.Unit})
				}
			}
			var selected int64
			for _, selection := range selections[r.ID] {
				q, e := core.Quantity(selection.Quantity)
				if e != nil || q <= 0 || !includes(r.RewardProductIDs, selection.ProductID) {
					return quote, core.Invalid("Regalo no válido.")
				}
				selected += q
				if selected > maxGift {
					return quote, core.Conflict("La cantidad de regalos supera el beneficio disponible.")
				}
				p, ok := products[selection.ProductID]
				if !ok {
					return quote, core.NotFound
				}
				l, e := appendLine(p, q, "gift", r.ID)
				if e != nil {
					return quote, e
				}
				l.auto = l.gross
				l.left = 0
				lines = append(lines, l)
				snapshot := r
				quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "promotion", LineID: l.line.ID, Label: r.Name + " · Regalo", AmountMinor: text(l.gross), Promotion: &snapshot})
				applied += l.gross
			}
			option.SelectedQuantity = core.QuantityText(selected)
			quote.RewardOptions = append(quote.RewardOptions, option)
			if selected > 0 {
				reserve(matched, r)
			}
			delete(selections, r.ID)
		default:
			return quote, core.Invalid("Oferta no compatible.")
		}
		if applied > 0 {
			reserve(matched, r)
		}
	}
	for _, selected := range selections {
		if len(selected) > 0 {
			return quote, core.Conflict("Los regalos seleccionados ya no están disponibles. Revisa las ofertas y retíralos del carrito.")
		}
	}
	if len(req.LineDiscounts) > 0 || req.TotalDiscount != nil {
		if !actor.CanDiscount() {
			return quote, core.Forbidden
		}
		if len(req.DiscountReason) < 3 {
			return quote, core.Invalid("Indica el motivo del descuento manual.")
		}
	}
	for _, manual := range req.LineDiscounts {
		var target *priced
		for _, l := range lines {
			if l.line.Kind == "regular" && l.line.ProductID == manual.ProductID {
				target = l
				break
			}
		}
		if target == nil {
			return quote, core.Invalid("El descuento manual no corresponde a una línea de venta.")
		}
		n, e := manual.Discount.Amount(target.left, target.qty)
		if e != nil {
			return quote, e
		}
		if n > target.left {
			return quote, core.Invalid("El descuento manual supera el importe disponible del producto.")
		}
		target.left -= n
		target.manual += n
		spec := manual.Discount
		quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "manual_line", LineID: target.line.ID, Label: "Descuento manual", AmountMinor: text(n), Specification: &spec, ActorID: actor.ID, ActorName: actor.Name, Reason: req.DiscountReason})
	}
	if req.TotalDiscount != nil {
		var remaining int64
		for _, l := range lines {
			remaining += l.left
		}
		n, e := req.TotalDiscount.Amount(remaining, 1000)
		if e != nil {
			return quote, e
		}
		if n > remaining {
			return quote, core.Invalid("El descuento manual supera el total disponible.")
		}
		distribute(n, lines, "order")
		spec := *req.TotalDiscount
		quote.Discounts = append(quote.Discounts, sales.AppliedDiscount{Kind: "manual_total", Label: "Descuento sobre el total", AmountMinor: text(n), Specification: &spec, ActorID: actor.ID, ActorName: actor.Name, Reason: req.DiscountReason})
	}
	var subtotal, automatic, manual, total int64
	demand := map[string]int64{}
	for _, l := range lines {
		p := products[l.line.ProductID]
		components := p.Components
		if p.StockMode == "own" {
			components = []catalog.Component{{ProductID: p.ID, Quantity: "1.000"}}
		}
		for _, component := range components {
			input, ok := products[component.ProductID]
			if !ok || input.StockMode != "own" {
				return quote, core.Conflict("La composición contiene un insumo inválido.")
			}
			factor, e := core.Quantity(component.Quantity)
			if e != nil {
				return quote, e
			}
			q, e := core.MulDiv(l.qty, factor, 1, false)
			if e != nil {
				return quote, e
			}
			if demand[input.ID] > 1_000_000_000_000_000-q {
				return quote, core.Invalid("El consumo excede el máximo de inventario.")
			}
			demand[input.ID] += q
			l.line.Consumptions = append(l.line.Consumptions, sales.Consumption{ProductID: input.ID, SKU: input.SKU, Name: input.Name, Unit: input.Unit, Quantity: core.StockText(q), Factor: component.Quantity, StockPolicy: input.StockPolicy})
		}
		l.line.GrossMinor = text(l.gross)
		l.line.PromotionDiscountMinor = text(l.auto)
		l.line.ManualDiscountMinor = text(l.manual)
		l.line.OrderDiscountMinor = text(l.order)
		l.line.TotalMinor = text(l.left)
		quote.Lines = append(quote.Lines, l.line)
		subtotal += l.gross
		automatic += l.auto
		manual += l.manual + l.order
		total += l.left
		if subtotal > 1_000_000_000_000_000 {
			return quote, core.Invalid("El importe de la venta excede el máximo.")
		}
	}
	quote.SubtotalMinor = text(subtotal)
	quote.PromotionDiscountMinor = text(automatic)
	quote.ManualDiscountMinor = text(manual)
	quote.TotalMinor = text(total)
	ids := []string{}
	for id := range demand {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := products[id]
		a := sales.Availability{ProductID: id, Name: p.Name, RequiredQuantity: core.StockText(demand[id]), Sufficient: p.StockPolicy != "block" || balances[id] >= demand[id]}
		if p.StockPolicy != "untracked" {
			v := core.StockText(balances[id])
			a.AvailableQuantity = &v
		}
		quote.Availability = append(quote.Availability, a)
	}
	dependencies := struct {
		Result     sales.Result
		Rewards    []sales.RewardOption
		Products   map[string]catalog.Product
		Rules      []promotions.Rule
		Actor      string
		Permission bool
	}{quote.Result, quote.RewardOptions, products, active, actor.ID, actor.CanDiscount()}
	raw, e := json.Marshal(dependencies)
	if e != nil {
		return quote, e
	}
	sum := sha256.Sum256(raw)
	quote.Fingerprint = hex.EncodeToString(sum[:])
	return quote, nil
}
