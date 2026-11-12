package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// Existing products can generate a second, stable internal code without changing
// their manufacturer identifiers. Allocation and product version share the UoW.
func (s *Service) GenerateBarcode(ctx context.Context, actor admin.User, id string) (catalog.Product, error) {
	var result catalog.Product
	if actor.Role != "admin" {
		return result, core.Forbidden
	}
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		product, e := p.Catalog.Product(ctx, id)
		if e != nil {
			return e
		}
		for _, b := range product.Barcodes {
			if b.Generated {
				result = product
				return nil
			}
		}
		for tries := 0; tries < 10; tries++ {
			n, e := rand.Int(rand.Reader, big.NewInt(1_000_000_000_000))
			if e != nil {
				return e
			}
			value := fmt.Sprintf("RZ%012d", n.Int64())
			if _, e = p.Catalog.ProductByBarcode(ctx, value); core.HasCode(e, "not_found") {
				product.Barcodes = append(product.Barcodes, catalog.Barcode{Value: value, Format: "CODE128", Generated: true, Primary: len(product.Barcodes) == 0})
				if e = product.Validate(); e != nil {
					return e
				}
				if e = p.Catalog.UpdateProduct(ctx, product, product.Version); e != nil {
					return e
				}
				product.Version++
				entry := auditEntry(actor.ID, "product.barcode_generate", id)
				entry.Details = map[string]any{"barcode": value, "product": product}
				if e = p.Audit.Append(ctx, entry); e != nil {
					return e
				}
				result = product
				return nil
			} else if e != nil {
				return e
			}
		}
		return core.Conflict("No se pudo asignar un código único. Inténtalo nuevamente.")
	})
	return result, e
}
