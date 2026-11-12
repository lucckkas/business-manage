package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"context"
	"sort"
	"strconv"
	"strings"
)

func validateComponents(ctx context.Context, p Ports, product catalog.Product) error {
	if product.StockMode != "components" {
		return nil
	}
	used, e := p.Catalog.HasDependents(ctx, product.ID)
	if e != nil {
		return e
	}
	if used {
		return core.Conflict("Un insumo utilizado por otro producto no puede tener composición.")
	}
	for _, c := range product.Components {
		input, e := p.Catalog.Product(ctx, c.ProductID)
		if e != nil {
			return e
		}
		if input.StockMode != "own" {
			return core.Invalid("Los insumos deben tener stock propio o estar sin seguimiento; no se admiten composiciones anidadas.")
		}
	}
	return nil
}
func (s *Service) CreateProduct(ctx context.Context, actor admin.User, product catalog.Product) (catalog.Product, error) {
	if actor.Role != "admin" {
		return product, core.Forbidden
	}
	for _, b := range product.Barcodes {
		if b.Generated {
			return product, core.Invalid("Los códigos internos se generan desde el servidor.")
		}
	}
	if e := product.Validate(); e != nil {
		return product, e
	}
	n, _ := core.Minor(product.PriceMinor)
	product.PriceMinor = strconv.FormatInt(n, 10)
	product.ID = core.ID()
	product.CreatedAt = core.Now()
	product.Version = 1
	sort.Slice(product.Components, func(i, j int) bool { return product.Components[i].ProductID < product.Components[j].ProductID })
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		if e := validateComponents(ctx, p, product); e != nil {
			return e
		}
		if e := p.Catalog.CreateProduct(ctx, product); e != nil {
			return e
		}
		if e := p.Inventory.Initialize(ctx, product.ID); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "product.create", product.ID)
		entry.Details = map[string]any{"product": product}
		return p.Audit.Append(ctx, entry)
	})
	return product, e
}

type ProductUpdate struct {
	catalog.Product
	CountedStock     *string `json:"countedStock,omitempty"`
	AdjustmentReason string  `json:"adjustmentReason,omitempty"`
}

func (s *Service) UpdateProduct(ctx context.Context, actor admin.User, id string, request ProductUpdate) (catalog.Product, error) {
	var result catalog.Product
	if actor.Role != "admin" {
		return result, core.Forbidden
	}
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		old, e := p.Catalog.Product(ctx, id)
		if e != nil {
			return e
		}
		product := request.Product
		if product.Barcodes == nil {
			product.Barcodes = append([]catalog.Barcode{}, old.Barcodes...)
		}
		if e = catalog.ValidateBarcodes(product.Barcodes); e != nil {
			return e
		}
		for i := range product.Barcodes {
			b := &product.Barcodes[i]
			valid := false
			for _, prior := range old.Barcodes {
				if prior.Generated && prior.Value == b.Value {
					valid = true
					b.Generated = true
				}
			}
			if b.Generated && (!valid || b.Format != "CODE128") {
				return core.Invalid("El código interno no puede modificarse manualmente.")
			}
		}
		product.ID = id
		product.CreatedAt = old.CreatedAt
		if product.Version != old.Version {
			return core.Conflict("El producto cambió. Recarga antes de guardar.")
		}
		if e = product.Validate(); e != nil {
			return e
		}
		if e = validateComponents(ctx, p, product); e != nil {
			return e
		}
		if product.Unit != old.Unit {
			history, e := p.Inventory.HasHistory(ctx, id)
			if e != nil {
				return e
			}
			sold, e := p.Sales.ProductSold(ctx, id)
			if e != nil {
				return e
			}
			used, e := p.Catalog.HasDependents(ctx, id)
			if e != nil {
				return e
			}
			if history || sold || used {
				return core.Conflict("La unidad no puede cambiar cuando hay historial o uso como insumo.")
			}
			rules, e := p.Promotions.All(ctx)
			if e != nil {
				return e
			}
			for _, r := range rules {
				if r.Kind != "discount" {
					for _, ids := range [][]string{r.ProductIDs, r.RewardProductIDs} {
						for _, pid := range ids {
							if pid == id {
								return core.Conflict("La unidad está utilizada en una oferta por cantidad.")
							}
						}
					}
				}
			}
		}
		current, e := p.Inventory.Stock(ctx, id)
		if e != nil {
			return e
		}
		stockBefore := current
		if product.StockMode == "own" && product.StockPolicy != "untracked" && (old.StockMode != "own" || old.StockPolicy == "untracked") && request.CountedStock == nil {
			return core.Invalid("Indica las existencias actuales al activar el seguimiento.")
		}
		if request.CountedStock != nil {
			target, e := core.StockQuantity(*request.CountedStock)
			if e != nil {
				return e
			}
			if product.StockMode == "components" && target != 0 {
				return core.Invalid("Para usar insumos, el saldo propio debe quedar en cero.")
			}
			if target < 0 && product.StockPolicy != "negative" {
				return core.Invalid("La política elegida no admite un saldo contado negativo.")
			}
			reason := strings.TrimSpace(request.AdjustmentReason)
			if len(reason) < 3 || len(reason) > 240 {
				return core.Invalid("Indica un motivo para el ajuste de existencias.")
			}
			if e = p.Inventory.Adjust(ctx, inventory.Adjustment{ID: core.ID(), ProductID: id, CountedQuantity: core.StockText(target), Reason: reason, CreatedAt: core.Now()}, actor.ID); e != nil {
				return e
			}
			current = target
		}
		if product.StockMode == "components" && current != 0 {
			return core.Conflict("Ajusta a cero el stock propio antes de cambiar a consumo por insumos.")
		}
		if product.StockMode == "own" && product.StockPolicy == "zero" && current < 0 {
			if e = p.Inventory.Adjust(ctx, inventory.Adjustment{ID: core.ID(), ProductID: id, CountedQuantity: "0.000000", Reason: "Cambio de política: mantener mínimo cero", CreatedAt: core.Now()}, actor.ID); e != nil {
				return e
			}
			current = 0
		}
		if product.StockMode == "own" && product.StockPolicy == "block" && current < 0 {
			return core.Conflict("Ajusta el saldo negativo antes de activar el bloqueo de faltantes.")
		}
		n, _ := core.Minor(product.PriceMinor)
		product.PriceMinor = strconv.FormatInt(n, 10)
		sort.Slice(product.Components, func(i, j int) bool { return product.Components[i].ProductID < product.Components[j].ProductID })
		if e = p.Catalog.UpdateProduct(ctx, product, old.Version); e != nil {
			return e
		}
		product.Version = old.Version + 1
		result = product
		entry := auditEntry(actor.ID, "product.update", id)
		entry.Details = map[string]any{"before": old, "after": product, "stockBefore": core.StockText(stockBefore), "stockAfter": core.StockText(current), "countedStock": request.CountedStock, "adjustmentReason": request.AdjustmentReason}
		return p.Audit.Append(ctx, entry)
	})
	return result, e
}
func (s *Service) ReceiveStock(ctx context.Context, actor admin.User, r inventory.Receipt) (inventory.Receipt, error) {
	if actor.Role != "admin" {
		return r, core.Forbidden
	}
	q, e := core.StockQuantity(r.Quantity)
	if e != nil {
		return r, e
	}
	if q <= 0 {
		return r, core.Invalid("La cantidad debe ser mayor que cero.")
	}
	r.Reason = strings.TrimSpace(r.Reason)
	if len(r.Reason) < 3 || len(r.Reason) > 240 {
		return r, core.Invalid("Indica un motivo de 3 a 240 caracteres.")
	}
	r.ID = core.ID()
	r.CreatedAt = core.Now()
	r.Quantity = core.StockText(q)
	e = s.Store.WithinTx(ctx, func(p Ports) error {
		product, e := p.Catalog.Product(ctx, r.ProductID)
		if e != nil {
			return e
		}
		if product.StockMode != "own" || product.StockPolicy == "untracked" {
			return core.Invalid("Activa el seguimiento de stock propio para ingresar existencias.")
		}
		before, e := p.Inventory.Stock(ctx, r.ProductID)
		if e != nil {
			return e
		}
		if e = p.Inventory.Receive(ctx, r, q, actor.ID); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "inventory.receive", r.ID)
		entry.Details = map[string]any{"receipt": r, "product": product, "stockBefore": core.StockText(before), "stockAfter": core.StockText(before + q)}
		return p.Audit.Append(ctx, entry)
	})
	return r, e
}
func (s *Service) AdjustStock(ctx context.Context, actor admin.User, a inventory.Adjustment) (inventory.Adjustment, error) {
	if actor.Role != "admin" {
		return a, core.Forbidden
	}
	q, e := core.StockQuantity(a.CountedQuantity)
	if e != nil {
		return a, e
	}
	a.Reason = strings.TrimSpace(a.Reason)
	if len(a.Reason) < 3 || len(a.Reason) > 240 {
		return a, core.Invalid("Indica un motivo de ajuste.")
	}
	a.ID = core.ID()
	a.CreatedAt = core.Now()
	a.CountedQuantity = core.StockText(q)
	e = s.Store.WithinTx(ctx, func(p Ports) error {
		product, e := p.Catalog.Product(ctx, a.ProductID)
		if e != nil {
			return e
		}
		if product.StockMode != "own" || product.StockPolicy == "untracked" {
			return core.Invalid("El producto no mantiene stock propio controlado.")
		}
		if q < 0 && product.StockPolicy != "negative" {
			return core.Invalid("La política no permite saldos negativos.")
		}
		before, e := p.Inventory.Stock(ctx, a.ProductID)
		if e != nil {
			return e
		}
		if e = p.Inventory.Adjust(ctx, a, actor.ID); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "inventory.adjust", a.ProductID)
		entry.Details = map[string]any{"before": core.StockText(before), "after": a.CountedQuantity, "reason": a.Reason, "product": product}
		return p.Audit.Append(ctx, entry)
	})
	return a, e
}
