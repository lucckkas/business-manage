// Package application coordinates business use cases through module-owned ports.
package application

import (
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/reporting"
	"business-manage/internal/modules/sales"
	"business-manage/internal/modules/treasury"
	"context"
	"log/slog"
	"time"
)

type Ports struct {
	Admin      admin.Repository
	Catalog    catalog.Repository
	Inventory  inventory.Repository
	Sales      sales.Repository
	Treasury   treasury.Repository
	Audit      audit.Repository
	Reports    reporting.Repository
	Promotions promotions.Repository
	POS        pos.Repository
}
type Store interface {
	Read() Ports
	WithinTx(context.Context, func(Ports) error) error
	ReadSnapshot(context.Context, func(Ports) error) error
}
type Service struct {
	Store   Store
	Now     func() time.Time
	Printer pos.Transport
	Log     *slog.Logger
}

func New(store Store) *Service { return &Service{Store: store, Now: time.Now} }
