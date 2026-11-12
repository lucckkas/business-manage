package catalog

import "business-manage/internal/core"

// Maximum returns sale thousandths. A nil limit means no blocking stock policy.
func Maximum(p Product, products map[string]Product, balances map[string]int64) (*string, error) {
	var maximum *int64
	limit := func(value int64) {
		if value < 0 {
			value = 0
		}
		if value > 1_000_000_000_000 {
			value = 1_000_000_000_000
		}
		if maximum == nil || value < *maximum {
			n := value
			maximum = &n
		}
	}
	if p.StockMode == "own" {
		if p.StockPolicy == "block" {
			limit(balances[p.ID] / 1000)
		}
	} else {
		for _, c := range p.Components {
			input, ok := products[c.ProductID]
			if !ok {
				return nil, core.NotFound
			}
			if input.StockPolicy != "block" {
				continue
			}
			factor, e := core.Quantity(c.Quantity)
			if e != nil || factor <= 0 {
				return nil, core.Invalid("Composición inválida.")
			}
			limit(balances[c.ProductID] / factor)
		}
	}
	if maximum == nil {
		return nil, nil
	}
	text := core.QuantityText(*maximum)
	return &text, nil
}
