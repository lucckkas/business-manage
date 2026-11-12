package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/catalog"
	"context"
)

func (r *repos) barcodes(ctx context.Context, id string) ([]catalog.Barcode, error) {
	out := []catalog.Barcode{}
	rows, e := r.db.QueryContext(ctx, "SELECT value,format,generated,is_primary FROM catalog_barcodes WHERE product_id=? ORDER BY is_primary DESC,value", id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var b catalog.Barcode
		if e = rows.Scan(&b.Value, &b.Format, &b.Generated, &b.Primary); e != nil {
			return out, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r *repos) saveBarcodes(ctx context.Context, p catalog.Product) error {
	if _, e := r.db.ExecContext(ctx, "DELETE FROM catalog_barcodes WHERE product_id=?", p.ID); e != nil {
		return e
	}
	for _, b := range p.Barcodes {
		if _, e := r.db.ExecContext(ctx, "INSERT INTO catalog_barcodes(value,product_id,format,generated,is_primary) VALUES(?,?,?,?,?)", b.Value, p.ID, b.Format, b.Generated, b.Primary); e != nil {
			mapped := translate(e)
			if core.HasCode(mapped, "conflict") {
				return core.Conflict("Un código de barras ya está asignado a otro producto.")
			}
			return mapped
		}
	}
	return nil
}
func (r *repos) ProductByBarcode(ctx context.Context, value string) (catalog.Product, error) {
	var id string
	e := r.db.QueryRowContext(ctx, "SELECT product_id FROM catalog_barcodes WHERE value=?", value).Scan(&id)
	if e != nil {
		return catalog.Product{}, translate(e)
	}
	return r.Product(ctx, id)
}
