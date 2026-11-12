// Package reporting defines read-only projections across module boundaries.
package reporting

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/sales"
	"business-manage/internal/modules/treasury"
	"context"
)

type Product struct {
	catalog.Product
	Stock             *string `json:"stock"`
	AvailableQuantity *string `json:"availableQuantity"`
}
type Summary struct {
	Products          int    `json:"products"`
	Sales             int    `json:"sales"`
	RevenueMinor      string `json:"revenueMinor"`
	CashMinor         string `json:"cashMinor"`
	CashIncomeMinor   string `json:"cashIncomeMinor"`
	CashExpenseMinor  string `json:"cashExpenseMinor"`
	OutOfStock        int    `json:"outOfStock"`
	TodaySales        int    `json:"todaySales"`
	TodayRevenueMinor string `json:"todayRevenueMinor"`
}
type Repository interface {
	ProductView(context.Context, string) (Product, error)
	Products(context.Context, core.Query) (core.Page[Product], error)
	Sales(context.Context, core.Query) (core.Page[sales.Sale], error)
	Cash(context.Context, core.Query) (core.Page[treasury.Movement], error)
	Audit(context.Context, core.Query) (core.Page[audit.Entry], error)
	Summary(context.Context, string, string) (Summary, error)
}
