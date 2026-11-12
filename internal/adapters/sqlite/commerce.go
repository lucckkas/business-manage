package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/sales"
	"context"
	"encoding/json"
)

func (r *repos) CreateProduct(ctx context.Context, p catalog.Product) error {
	_, e := r.db.ExecContext(ctx, "INSERT INTO catalog_products(id,sku,name,unit,price_minor,created_at,sellable,stock_mode,stock_policy,version) VALUES(?,?,?,?,?,?,?,?,?,1)", p.ID, p.SKU, p.Name, p.Unit, p.PriceMinor, p.CreatedAt, p.Sellable, p.StockMode, p.StockPolicy)
	if e != nil {
		return translate(e)
	}
	if e = r.saveComponents(ctx, p); e != nil {
		return e
	}
	return r.saveBarcodes(ctx, p)
}
func (r *repos) saveComponents(ctx context.Context, p catalog.Product) error {
	if _, e := r.db.ExecContext(ctx, "DELETE FROM catalog_components WHERE product_id=?", p.ID); e != nil {
		return e
	}
	for _, c := range p.Components {
		q, _ := core.Quantity(c.Quantity)
		if _, e := r.db.ExecContext(ctx, "INSERT INTO catalog_components VALUES(?,?,?)", p.ID, c.ProductID, q); e != nil {
			return e
		}
	}
	return nil
}
func (r *repos) UpdateProduct(ctx context.Context, p catalog.Product, version int) error {
	result, e := r.db.ExecContext(ctx, "UPDATE catalog_products SET sku=?,name=?,unit=?,price_minor=?,sellable=?,stock_mode=?,stock_policy=?,version=version+1 WHERE id=? AND version=?", p.SKU, p.Name, p.Unit, p.PriceMinor, p.Sellable, p.StockMode, p.StockPolicy, p.ID, version)
	if e != nil {
		return translate(e)
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return core.Conflict("El producto cambió. Recarga sus datos antes de editar.")
	}
	if e = r.saveComponents(ctx, p); e != nil {
		return e
	}
	return r.saveBarcodes(ctx, p)
}
func (r *repos) Product(ctx context.Context, id string) (catalog.Product, error) {
	var p catalog.Product
	e := r.db.QueryRowContext(ctx, "SELECT id,sku,name,unit,price_minor,created_at,sellable,stock_mode,stock_policy,version FROM catalog_products WHERE id=?", id).Scan(&p.ID, &p.SKU, &p.Name, &p.Unit, &p.PriceMinor, &p.CreatedAt, &p.Sellable, &p.StockMode, &p.StockPolicy, &p.Version)
	if e != nil {
		return p, translate(e)
	}
	p.Components = []catalog.Component{}
	rows, e := r.db.QueryContext(ctx, "SELECT component_id,factor_milli FROM catalog_components WHERE product_id=? ORDER BY component_id", id)
	if e != nil {
		return p, e
	}
	for rows.Next() {
		var c catalog.Component
		var q int64
		if e = rows.Scan(&c.ProductID, &q); e != nil {
			rows.Close()
			return p, e
		}
		c.Quantity = core.QuantityText(q)
		p.Components = append(p.Components, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	p.Barcodes, e = r.barcodes(ctx, id)
	return p, e
}
func (r *repos) HasDependents(ctx context.Context, id string) (bool, error) {
	var n int
	e := r.db.QueryRowContext(ctx, "SELECT count(*) FROM catalog_components WHERE component_id=?", id).Scan(&n)
	return n > 0, e
}
func (r *repos) Initialize(ctx context.Context, id string) error {
	_, e := r.db.ExecContext(ctx, "INSERT INTO inventory_stock VALUES(?,0)", id)
	return e
}
func (r *repos) Stock(ctx context.Context, id string) (int64, error) {
	var q int64
	e := r.db.QueryRowContext(ctx, "SELECT qty_micro FROM inventory_stock WHERE product_id=?", id).Scan(&q)
	return q, translate(e)
}
func (r *repos) HasHistory(ctx context.Context, id string) (bool, error) {
	var n int
	e := r.db.QueryRowContext(ctx, "SELECT count(*) FROM inventory_movements WHERE product_id=?", id).Scan(&n)
	return n > 0, e
}
func (r *repos) Receive(ctx context.Context, receipt inventory.Receipt, q int64, actor string) error {
	if _, e := r.db.ExecContext(ctx, "INSERT INTO inventory_receipts(id,product_id,qty_micro,reason,actor_id,created_at) VALUES(?,?,?,?,?,?)", receipt.ID, receipt.ProductID, q, receipt.Reason, actor, receipt.CreatedAt); e != nil {
		return e
	}
	if _, e := r.db.ExecContext(ctx, "UPDATE inventory_stock SET qty_micro=qty_micro+? WHERE product_id=?", q, receipt.ProductID); e != nil {
		return e
	}
	_, e := r.db.ExecContext(ctx, "INSERT INTO inventory_movements(id,product_id,qty_micro,kind,receipt_id,actor_id,reason,created_at) VALUES(?,?,?,'receipt',?,?,?,?)", core.ID(), receipt.ProductID, q, receipt.ID, actor, receipt.Reason, receipt.CreatedAt)
	return e
}
func (r *repos) Consume(ctx context.Context, id string, q int64, policy, sale, actor, date string) error {
	if policy == "untracked" {
		return nil
	}
	current, e := r.Stock(ctx, id)
	if e != nil {
		return e
	}
	next := current - q
	if policy == "block" && next < 0 {
		return core.Conflict("Existencias insuficientes. Revisa los productos e insumos.")
	}
	compensation := int64(0)
	if policy == "zero" && next < 0 {
		compensation = -next
		next = 0
	}
	result, e := r.db.ExecContext(ctx, "UPDATE inventory_stock SET qty_micro=? WHERE product_id=? AND qty_micro=?", next, id, current)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return core.Conflict("Las existencias cambiaron. Vuelve a intentarlo.")
	}
	if _, e = r.db.ExecContext(ctx, "INSERT INTO inventory_movements(id,product_id,qty_micro,kind,sale_id,actor_id,created_at) VALUES(?,?,?,'sale',?,?,?)", core.ID(), id, -q, sale, actor, date); e != nil {
		return e
	}
	if compensation > 0 {
		_, e = r.db.ExecContext(ctx, "INSERT INTO inventory_movements(id,product_id,qty_micro,kind,sale_id,actor_id,reason,created_at) VALUES(?,?,?,'compensation',?,?,'Faltante: mantener mínimo cero',?)", core.ID(), id, compensation, sale, actor, date)
	}
	return e
}
func (r *repos) Adjust(ctx context.Context, a inventory.Adjustment, actor string) error {
	current, e := r.Stock(ctx, a.ProductID)
	if e != nil {
		return e
	}
	target, e := core.StockQuantity(a.CountedQuantity)
	if e != nil {
		return e
	}
	delta := target - current
	if delta == 0 {
		return nil
	}
	if _, e = r.db.ExecContext(ctx, "UPDATE inventory_stock SET qty_micro=? WHERE product_id=?", target, a.ProductID); e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "INSERT INTO inventory_movements(id,product_id,qty_micro,kind,actor_id,reason,created_at) VALUES(?,?,?,'adjustment',?,?,?)", a.ID, a.ProductID, delta, actor, a.Reason, a.CreatedAt)
	return e
}
func (r *repos) CreateSale(ctx context.Context, s *sales.Sale) error {
	result, e := r.db.ExecContext(ctx, "INSERT INTO sales_documents(id,total_minor,actor_id,created_at,idempotency_key,request_hash,print_job_id) VALUES(?,?,?,?,?,?,?)", s.ID, s.TotalMinor, s.ActorID, s.CreatedAt, s.IdempotencyKey, s.RequestHash, s.PrintJobID)
	if e != nil {
		return translate(e)
	}
	s.Number, e = result.LastInsertId()
	if e != nil {
		return e
	}
	for _, line := range s.Lines {
		q, _ := core.Quantity(line.Quantity)
		if _, e = r.db.ExecContext(ctx, "INSERT INTO sales_lines(id,sale_id,product_id,sku,name,unit,qty_milli,price_minor,total_minor,kind,promotion_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)", line.ID, s.ID, line.ProductID, line.SKU, line.Name, line.Unit, q, line.PriceMinor, line.TotalMinor, line.Kind, line.PromotionID); e != nil {
			return e
		}
	}
	snapshot, e := json.Marshal(s)
	if e != nil {
		return e
	}
	company, e := json.Marshal(s.Company)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "UPDATE sales_documents SET details_json=?,company_json=? WHERE id=?", string(snapshot), string(company), s.ID)
	return e
}
func (r *repos) SaleByKey(ctx context.Context, key string) (sales.Sale, error) {
	return r.loadSale(ctx, "idempotency_key", key)
}
func (r *repos) Sale(ctx context.Context, id string) (sales.Sale, error) {
	return r.loadSale(ctx, "id", id)
}
func (r *repos) ProductSold(ctx context.Context, id string) (bool, error) {
	var n int
	e := r.db.QueryRowContext(ctx, "SELECT count(*) FROM sales_lines WHERE product_id=?", id).Scan(&n)
	return n > 0, e
}
func (r *repos) loadSale(ctx context.Context, column, value string) (sales.Sale, error) {
	var s sales.Sale
	var snapshot, company string
	e := r.db.QueryRowContext(ctx, "SELECT id,number,total_minor,actor_id,created_at,idempotency_key,request_hash,details_json,company_json FROM sales_documents WHERE "+column+"=?", value).Scan(&s.ID, &s.Number, &s.TotalMinor, &s.ActorID, &s.CreatedAt, &s.IdempotencyKey, &s.RequestHash, &snapshot, &company)
	if e != nil {
		return s, translate(e)
	}
	if snapshot != "" {
		e = json.Unmarshal([]byte(snapshot), &s)
		return s, e
	}
	if e = json.Unmarshal([]byte(company), &s.Company); e != nil {
		return s, e
	}
	s.SubtotalMinor = s.TotalMinor
	s.PromotionDiscountMinor = "0"
	s.ManualDiscountMinor = "0"
	s.Discounts = []sales.AppliedDiscount{}
	s.Lines = []sales.Line{}
	rows, e := r.db.QueryContext(ctx, "SELECT id,product_id,sku,name,unit,qty_milli,price_minor,total_minor FROM sales_lines WHERE sale_id=? ORDER BY product_id", s.ID)
	if e != nil {
		return s, e
	}
	defer rows.Close()
	for rows.Next() {
		var l sales.Line
		var q int64
		if e = rows.Scan(&l.ID, &l.ProductID, &l.SKU, &l.Name, &l.Unit, &q, &l.PriceMinor, &l.TotalMinor); e != nil {
			return s, e
		}
		l.Quantity = core.QuantityText(q)
		l.Kind = "regular"
		l.GrossMinor = l.TotalMinor
		l.PromotionDiscountMinor = "0"
		l.ManualDiscountMinor = "0"
		l.OrderDiscountMinor = "0"
		l.Consumptions = []sales.Consumption{{ProductID: l.ProductID, SKU: l.SKU, Name: l.Name, Unit: l.Unit, Quantity: core.StockText(q * 1000), Factor: "1.000", StockPolicy: "block"}}
		s.Lines = append(s.Lines, l)
	}
	return s, rows.Err()
}
