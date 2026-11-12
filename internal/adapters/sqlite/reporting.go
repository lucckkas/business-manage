package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/reporting"
	"business-manage/internal/modules/sales"
	"context"
)

func (r *repos) Products(ctx context.Context, q core.Query) (core.Page[reporting.Product], error) {
	q = q.Normalized()
	out := core.Page[reporting.Product]{Items: []reporting.Product{}, Page: q.Page, PageSize: q.PageSize}
	filter := like(q.Search)
	const where = " FROM catalog_products WHERE (name LIKE ? ESCAPE '!' OR sku LIKE ? ESCAPE '!' OR EXISTS(SELECT 1 FROM catalog_barcodes b WHERE b.product_id=catalog_products.id AND b.value LIKE ? ESCAPE '!')) AND (?=0 OR sellable=1) AND (?=0 OR stock_mode='own')"
	if err := r.db.QueryRowContext(ctx, "SELECT count(*)"+where, filter, filter, filter, q.SellableOnly, q.OwnOnly).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT id"+where+" ORDER BY name COLLATE NOCASE,id LIMIT ? OFFSET ?", filter, filter, filter, q.SellableOnly, q.OwnOnly, q.PageSize, q.Offset())
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		view, e := r.ProductView(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, view)
	}
	return out, nil
}
func (r *repos) ProductView(ctx context.Context, id string) (reporting.Product, error) {
	product, e := r.Product(ctx, id)
	view := reporting.Product{Product: product}
	if e != nil {
		return view, e
	}
	products := map[string]catalog.Product{id: product}
	balance, e := r.Stock(ctx, id)
	if e != nil {
		return view, e
	}
	balances := map[string]int64{id: balance}
	if product.StockMode == "own" && product.StockPolicy != "untracked" {
		v := core.StockText(balance)
		view.Stock = &v
	}
	for _, c := range product.Components {
		input, e := r.Product(ctx, c.ProductID)
		if e != nil {
			return view, e
		}
		products[input.ID] = input
		balances[input.ID], e = r.Stock(ctx, input.ID)
		if e != nil {
			return view, e
		}
	}
	view.AvailableQuantity, e = catalog.Maximum(product, products, balances)
	return view, e
}

func (r *repos) Sales(ctx context.Context, q core.Query) (core.Page[sales.Sale], error) {
	q = q.Normalized()
	out := core.Page[sales.Sale]{Items: []sales.Sale{}, Page: q.Page, PageSize: q.PageSize}
	filter := like(q.Search)
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM sales_documents WHERE CAST(number AS TEXT) LIKE ? ESCAPE '!'", filter).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM sales_documents WHERE CAST(number AS TEXT) LIKE ? ESCAPE '!' ORDER BY created_at DESC,number DESC LIMIT ? OFFSET ?", filter, q.PageSize, q.Offset())
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		sale, e := r.Sale(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, sale)
	}
	return out, nil
}
func (r *repos) Summary(ctx context.Context, start, end string) (reporting.Summary, error) {
	var s reporting.Summary
	err := r.db.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM catalog_products),
 (SELECT count(*) FROM sales_documents),
 (SELECT CAST(COALESCE(sum(total_minor),0) AS TEXT) FROM sales_documents),
 (SELECT CAST(COALESCE(sum(amount_minor),0) AS TEXT) FROM treasury_movements),
 (SELECT CAST(COALESCE(sum(amount_minor),0) AS TEXT) FROM treasury_movements WHERE kind='sale'),
 (SELECT CAST(-COALESCE(sum(amount_minor),0) AS TEXT) FROM treasury_movements WHERE kind='expense'),
 (SELECT count(*) FROM inventory_stock s JOIN catalog_products p ON p.id=s.product_id WHERE p.stock_mode='own' AND p.stock_policy!='untracked' AND s.qty_micro<=0),
 (SELECT count(*) FROM sales_documents WHERE created_at>=? AND created_at<?),
 (SELECT CAST(COALESCE(sum(total_minor),0) AS TEXT) FROM sales_documents WHERE created_at>=? AND created_at<?)`, start, end, start, end).Scan(&s.Products, &s.Sales, &s.RevenueMinor, &s.CashMinor, &s.CashIncomeMinor, &s.CashExpenseMinor, &s.OutOfStock, &s.TodaySales, &s.TodayRevenueMinor)
	return s, err
}
