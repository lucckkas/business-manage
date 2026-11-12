package sales

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"context"
)

type LineRequest struct {
	ProductID string `json:"productId"`
	Quantity  string `json:"quantity"`
}
type Request struct {
	Lines            []LineRequest     `json:"lines"`
	LineDiscounts    []LineDiscount    `json:"lineDiscounts,omitempty"`
	TotalDiscount    *core.Discount    `json:"totalDiscount,omitempty"`
	DiscountReason   string            `json:"discountReason,omitempty"`
	Rewards          []RewardSelection `json:"rewards,omitempty"`
	QuoteFingerprint string            `json:"quoteFingerprint,omitempty"`
}
type Line struct {
	ID                     string        `json:"id"`
	Kind                   string        `json:"kind"`
	PromotionID            string        `json:"promotionId,omitempty"`
	GrossMinor             string        `json:"grossMinor"`
	PromotionDiscountMinor string        `json:"promotionDiscountMinor"`
	ManualDiscountMinor    string        `json:"manualDiscountMinor"`
	OrderDiscountMinor     string        `json:"orderDiscountMinor"`
	Consumptions           []Consumption `json:"consumptions"`
	ProductID              string        `json:"productId"`
	SKU                    string        `json:"sku"`
	Name                   string        `json:"name"`
	Unit                   string        `json:"unit"`
	Quantity               string        `json:"quantity"`
	PriceMinor             string        `json:"priceMinor"`
	TotalMinor             string        `json:"totalMinor"`
}
type Sale struct {
	Result
	Company        admin.Company `json:"company"`
	ID             string        `json:"id"`
	Number         int64         `json:"number"`
	CreatedAt      string        `json:"createdAt"`
	ActorID        string        `json:"actorId"`
	IdempotencyKey string        `json:"-"`
	RequestHash    string        `json:"-"`
	PrintJobID     string        `json:"printJobId,omitempty"`
}
type Repository interface {
	SaleByKey(context.Context, string) (Sale, error)
	Sale(context.Context, string) (Sale, error)
	CreateSale(context.Context, *Sale) error
	ProductSold(context.Context, string) (bool, error)
}
