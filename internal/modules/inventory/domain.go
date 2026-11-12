package inventory

import "context"

type Receipt struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	Quantity  string `json:"quantity"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}
type Repository interface {
	Initialize(context.Context, string) error
	Receive(context.Context, Receipt, int64, string) error
	Consume(context.Context, string, int64, string, string, string, string) error
	Stock(context.Context, string) (int64, error)
	Adjust(context.Context, Adjustment, string) error
	HasHistory(context.Context, string) (bool, error)
}
type Adjustment struct {
	ID              string `json:"id"`
	ProductID       string `json:"productId"`
	CountedQuantity string `json:"countedQuantity"`
	Reason          string `json:"reason"`
	CreatedAt       string `json:"createdAt"`
}
