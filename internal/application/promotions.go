package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/promotions"
	"context"
	"sort"
)

func (s *Service) SavePromotion(ctx context.Context, actor admin.User, id string, rule promotions.Rule) (promotions.Rule, error) {
	if actor.Role != "admin" {
		return rule, core.Forbidden
	}
	expected := rule.Version
	if id == "" {
		rule.ID = core.ID()
		expected = 0
		rule.Version = 1
	} else {
		rule.ID = id
		rule.Version = expected + 1
	}
	if e := rule.Validate(); e != nil {
		return rule, e
	}
	sort.Strings(rule.ProductIDs)
	sort.Strings(rule.RewardProductIDs)
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		var before *promotions.Rule
		if expected > 0 {
			if prior, e := p.Promotions.Rule(ctx, id); e != nil {
				return e
			} else {
				before = &prior
			}
		}
		for _, ids := range [][]string{rule.ProductIDs, rule.RewardProductIDs} {
			unit := ""
			for _, pid := range ids {
				product, e := p.Catalog.Product(ctx, pid)
				if e != nil {
					return e
				}
				if !product.Sellable {
					return core.Invalid("Las ofertas deben utilizar productos disponibles para venta.")
				}
				if rule.Kind != "discount" {
					if unit != "" && unit != product.Unit {
						return core.Invalid("Los productos de cada grupo deben tener la misma unidad de medida.")
					}
					unit = product.Unit
				}
			}
		}
		if e := p.Promotions.Save(ctx, rule, expected); e != nil {
			return e
		}
		action := "promotion.create"
		if expected > 0 {
			action = "promotion.update"
		}
		entry := auditEntry(actor.ID, action, rule.ID)
		entry.Details = map[string]any{"promotion": rule, "before": before, "after": rule}
		return p.Audit.Append(ctx, entry)
	})
	return rule, e
}
