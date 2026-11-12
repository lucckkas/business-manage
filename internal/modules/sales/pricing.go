package sales

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/promotions"
)

type LineDiscount struct {
	ProductID string        `json:"productId"`
	Discount  core.Discount `json:"discount"`
}
type RewardSelection struct {
	PromotionID string `json:"promotionId"`
	ProductID   string `json:"productId"`
	Quantity    string `json:"quantity"`
}
type Consumption struct {
	ProductID   string `json:"productId"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Quantity    string `json:"quantity"`
	Factor      string `json:"factor"`
	StockPolicy string `json:"stockPolicy"`
}
type AppliedDiscount struct {
	Kind          string           `json:"kind"`
	LineID        string           `json:"lineId,omitempty"`
	Label         string           `json:"label"`
	AmountMinor   string           `json:"amountMinor"`
	Specification *core.Discount   `json:"specification,omitempty"`
	Promotion     *promotions.Rule `json:"promotion,omitempty"`
	ActorID       string           `json:"actorId,omitempty"`
	ActorName     string           `json:"actorName,omitempty"`
	Reason        string           `json:"reason,omitempty"`
}
type Result struct {
	Lines                  []Line            `json:"lines"`
	SubtotalMinor          string            `json:"subtotalMinor"`
	PromotionDiscountMinor string            `json:"promotionDiscountMinor"`
	ManualDiscountMinor    string            `json:"manualDiscountMinor"`
	TotalMinor             string            `json:"totalMinor"`
	Discounts              []AppliedDiscount `json:"discounts"`
}
type Availability struct {
	ProductID         string  `json:"productId"`
	Name              string  `json:"name"`
	RequiredQuantity  string  `json:"requiredQuantity"`
	AvailableQuantity *string `json:"availableQuantity"`
	Sufficient        bool    `json:"sufficient"`
}
type RewardProduct struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Unit      string `json:"unit"`
}
type RewardOption struct {
	PromotionID      string          `json:"promotionId"`
	Name             string          `json:"name"`
	MaximumQuantity  string          `json:"maximumQuantity"`
	SelectedQuantity string          `json:"selectedQuantity"`
	Products         []RewardProduct `json:"products"`
}
type Quote struct {
	Result
	Fingerprint   string         `json:"fingerprint"`
	RewardOptions []RewardOption `json:"rewardOptions"`
	Availability  []Availability `json:"availability"`
}
