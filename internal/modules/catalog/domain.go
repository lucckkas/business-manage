package catalog

import (
	"business-manage/internal/core"
	"context"
	"strings"
)

type Product struct {
	ID          string      `json:"id"`
	SKU         string      `json:"sku"`
	Name        string      `json:"name"`
	Unit        string      `json:"unit"`
	PriceMinor  string      `json:"priceMinor"`
	CreatedAt   string      `json:"createdAt"`
	Sellable    bool        `json:"sellable"`
	StockMode   string      `json:"stockMode"`
	StockPolicy string      `json:"stockPolicy"`
	Components  []Component `json:"components"`
	Version     int         `json:"version"`
	Barcodes    []Barcode   `json:"barcodes"`
}
type Component struct {
	ProductID string `json:"productId"`
	Quantity  string `json:"quantity"`
}

func (p *Product) Validate() error {
	if p.Barcodes == nil {
		p.Barcodes = []Barcode{}
	}
	if e := ValidateBarcodes(p.Barcodes); e != nil {
		return e
	}
	if p.StockMode == "" {
		p.StockMode = "own"
		p.StockPolicy = "block"
		p.Sellable = true
	}
	if p.StockPolicy == "" {
		p.StockPolicy = "block"
	}
	if p.Components == nil {
		p.Components = []Component{}
	}
	if p.StockMode != "own" && p.StockMode != "components" {
		return core.Invalid("Tipo de inventario inválido.")
	}
	switch p.StockPolicy {
	case "block", "negative", "zero", "untracked":
	default:
		return core.Invalid("Política de existencias inválida.")
	}
	if len(p.Components) > 100 {
		return core.Invalid("El producto admite hasta cien insumos.")
	}
	if p.StockMode == "own" && len(p.Components) > 0 {
		return core.Invalid("Un producto con stock propio no tiene composición.")
	}
	if p.StockMode == "components" && len(p.Components) == 0 {
		return core.Invalid("Agrega al menos un insumo.")
	}
	seen := map[string]bool{}
	for i, c := range p.Components {
		if c.ProductID == p.ID || seen[c.ProductID] {
			return core.Invalid("Los insumos no pueden repetirse ni apuntar al propio producto.")
		}
		q, e := core.Quantity(c.Quantity)
		if e != nil {
			return e
		}
		if q <= 0 {
			return core.Invalid("El consumo por unidad debe ser positivo.")
		}
		p.Components[i].Quantity = core.QuantityText(q)
		seen[c.ProductID] = true
	}
	p.SKU = strings.ToUpper(strings.TrimSpace(p.SKU))
	p.Name = strings.TrimSpace(p.Name)
	p.Unit = strings.TrimSpace(p.Unit)
	if len(p.SKU) < 1 || len(p.SKU) > 64 || len(p.Name) < 2 || len(p.Name) > 160 {
		return core.Invalid("Indica un código (hasta 64 caracteres) y un nombre (2 a 160 caracteres).")
	}
	if p.Unit != "un" && p.Unit != "kg" && p.Unit != "lt" && p.Unit != "mt" {
		return core.Invalid("Selecciona una unidad válida.")
	}
	_, err := core.Minor(p.PriceMinor)
	return err
}

type Repository interface {
	ProductByBarcode(context.Context, string) (Product, error)
	CreateProduct(context.Context, Product) error
	Product(context.Context, string) (Product, error)
	UpdateProduct(context.Context, Product, int) error
	HasDependents(context.Context, string) (bool, error)
}
