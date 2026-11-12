package core

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func Fixed(value string, scale int, signed bool) (int64, error) {
	negative := strings.HasPrefix(value, "-")
	if negative {
		if !signed {
			return 0, Invalid("El valor no puede ser negativo.")
		}
		value = value[1:]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || strings.Trim(parts[0], "0123456789") != "" {
		return 0, Invalid("Valor decimal inválido.")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > scale || strings.Trim(fraction, "0123456789") != "" {
			return 0, Invalid(fmt.Sprintf("Utiliza hasta %d decimales.", scale))
		}
	}
	for len(fraction) < scale {
		fraction += "0"
	}
	n, e := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if e != nil || n > 1_000_000_000_000_000 {
		return 0, Invalid("El valor excede el máximo permitido.")
	}
	if negative {
		n = -n
	}
	return n, nil
}
func FixedText(n int64, scale int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	factor := int64(1)
	for i := 0; i < scale; i++ {
		factor *= 10
	}
	return fmt.Sprintf("%s%d.%0*d", sign, n/factor, scale, n%factor)
}
func StockQuantity(v string) (int64, error) { return Fixed(v, 6, true) }
func StockText(v int64) string              { return FixedText(v, 6) }
func MulDiv(a, b, d int64, round bool) (int64, error) {
	n := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	if round {
		n.Add(n, big.NewInt(d/2))
	}
	n.Div(n, big.NewInt(d))
	if !n.IsInt64() || n.Sign() < 0 || n.Int64() > 1_000_000_000_000_000 {
		return 0, Invalid("El cálculo excede el máximo permitido.")
	}
	return n.Int64(), nil
}

type Discount struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Basis string `json:"basis,omitempty"`
}

func (d *Discount) Normalize() error {
	var n int64
	var e error
	switch d.Type {
	case "percentage":
		n, e = Fixed(d.Value, 2, false)
		if e == nil && n > 10000 {
			return Invalid("El porcentaje debe estar entre 0 y 100.")
		}
		d.Value = FixedText(n, 2)
		d.Basis = "line"
	case "amount":
		n, e = Minor(d.Value)
		d.Value = strconv.FormatInt(n, 10)
		if d.Basis == "" {
			d.Basis = "line"
		}
		if d.Basis != "line" && d.Basis != "unit" {
			return Invalid("Indica si el descuento es por línea o por unidad.")
		}
	default:
		return Invalid("Tipo de descuento inválido.")
	}
	return e
}
func (d Discount) Amount(base, q int64) (int64, error) {
	switch d.Type {
	case "percentage":
		n, e := Fixed(d.Value, 2, false)
		if e != nil {
			return 0, e
		}
		return MulDiv(base, n, 10000, true)
	case "amount":
		n, e := Minor(d.Value)
		if e != nil {
			return 0, e
		}
		if d.Basis == "unit" {
			return MulDiv(n, q, 1000, true)
		}
		return n, nil
	}
	return 0, Invalid("Tipo de descuento inválido.")
}

// CappedAmount limits automatic benefits before enforcing monetary bounds, so a
// large per-unit offer can still safely reduce a small-priced line to zero.
func (d Discount) CappedAmount(base, q int64) (int64, error) {
	if d.Type == "amount" && d.Basis == "unit" {
		n, e := Minor(d.Value)
		if e != nil {
			return 0, e
		}
		v := new(big.Int).Mul(big.NewInt(n), big.NewInt(q))
		v.Add(v, big.NewInt(500)).Div(v, big.NewInt(1000))
		if v.Cmp(big.NewInt(base)) >= 0 {
			return base, nil
		}
		return v.Int64(), nil
	}
	n, e := d.Amount(base, q)
	return min(n, base), e
}
