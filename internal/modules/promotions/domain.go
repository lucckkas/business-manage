// Package promotions owns declarative rules; the pricing package evaluates them.
package promotions

import (
	"business-manage/internal/core"
	"context"
	"strings"
	"time"
)

type Rule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Kind             string         `json:"kind"`
	Scope            string         `json:"scope"`
	ProductIDs       []string       `json:"productIds"`
	RewardProductIDs []string       `json:"rewardProductIds"`
	Discount         *core.Discount `json:"discount,omitempty"`
	BuyQuantity      string         `json:"buyQuantity,omitempty"`
	PayQuantity      string         `json:"payQuantity,omitempty"`
	GiftQuantity     string         `json:"giftQuantity,omitempty"`
	StartsAt         *string        `json:"startsAt"`
	EndsAt           *string        `json:"endsAt"`
	Enabled          bool           `json:"enabled"`
	Stackable        bool           `json:"stackable"`
	Priority         int            `json:"priority"`
	Version          int            `json:"version"`
}

func (r Rule) Active(at time.Time) bool {
	if !r.Enabled {
		return false
	}
	if r.StartsAt != nil {
		v, e := time.Parse(time.RFC3339Nano, *r.StartsAt)
		if e != nil || at.Before(v) {
			return false
		}
	}
	if r.EndsAt != nil {
		v, e := time.Parse(time.RFC3339Nano, *r.EndsAt)
		if e != nil || !at.Before(v) {
			return false
		}
	}
	return true
}
func (r *Rule) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if len(r.Name) < 2 || len(r.Name) > 160 {
		return core.Invalid("Indica un nombre de 2 a 160 caracteres.")
	}
	if r.Priority < 0 || r.Priority > 100000 {
		return core.Invalid("Prioridad fuera de rango.")
	}
	if r.Scope != "products" && r.Scope != "cart" {
		return core.Invalid("Ámbito de oferta inválido.")
	}
	if r.ProductIDs == nil {
		r.ProductIDs = []string{}
	}
	if r.RewardProductIDs == nil {
		r.RewardProductIDs = []string{}
	}
	if (r.Scope == "products" && len(r.ProductIDs) == 0) || len(r.ProductIDs) > 100 || len(r.RewardProductIDs) > 100 {
		return core.Invalid("Selecciona entre uno y cien productos por grupo.")
	}
	for _, ids := range [][]string{r.ProductIDs, r.RewardProductIDs} {
		seen := map[string]bool{}
		for _, id := range ids {
			if len(id) != 32 || seen[id] {
				return core.Invalid("Productos inválidos o repetidos en la oferta.")
			}
			seen[id] = true
		}
	}
	var start, end time.Time
	var e error
	if r.StartsAt != nil {
		start, e = time.Parse(time.RFC3339Nano, *r.StartsAt)
		if e != nil {
			return core.Invalid("Inicio inválido; indica fecha, hora y zona.")
		}
		v := start.UTC().Format(time.RFC3339Nano)
		r.StartsAt = &v
	}
	if r.EndsAt != nil {
		end, e = time.Parse(time.RFC3339Nano, *r.EndsAt)
		if e != nil {
			return core.Invalid("Fin inválido; indica fecha, hora y zona.")
		}
		v := end.UTC().Format(time.RFC3339Nano)
		r.EndsAt = &v
	}
	if !start.IsZero() && !end.IsZero() && !end.After(start) {
		return core.Invalid("El fin debe ser posterior al inicio.")
	}
	switch r.Kind {
	case "discount":
		if r.Discount == nil {
			return core.Invalid("Configura el descuento.")
		}
		if e = r.Discount.Normalize(); e != nil {
			return e
		}
		if r.Scope == "cart" && r.Discount.Basis == "unit" {
			return core.Invalid("El descuento total no es por unidad.")
		}
	case "buy_pay", "buy_get":
		if r.Scope != "products" {
			return core.Invalid("Las ofertas por cantidad requieren productos.")
		}
		buy, e := core.Quantity(r.BuyQuantity)
		if e != nil || buy <= 0 {
			return core.Invalid("La cantidad de compra debe ser positiva.")
		}
		r.BuyQuantity = core.QuantityText(buy)
		if r.Kind == "buy_pay" {
			pay, e := core.Quantity(r.PayQuantity)
			if e != nil || pay <= 0 || pay >= buy {
				return core.Invalid("La cantidad pagada debe ser positiva y menor que la entregada.")
			}
			r.PayQuantity = core.QuantityText(pay)
		} else {
			gift, e := core.Quantity(r.GiftQuantity)
			if e != nil || gift <= 0 || len(r.RewardProductIDs) == 0 {
				return core.Invalid("Configura cantidad y productos de regalo.")
			}
			r.GiftQuantity = core.QuantityText(gift)
		}
	default:
		return core.Invalid("Tipo de oferta inválido.")
	}
	return nil
}

type Repository interface {
	Rules(context.Context, core.Query) (core.Page[Rule], error)
	All(context.Context) ([]Rule, error)
	Rule(context.Context, string) (Rule, error)
	Save(context.Context, Rule, int) error
}
