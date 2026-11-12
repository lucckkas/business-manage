package treasury

import "context"

type Movement struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	Reference      string  `json:"reference"`
	SaleID         string  `json:"saleId,omitempty"`
	SaleNumber     int64   `json:"saleNumber,omitempty"`
	ExpenseNumber  int64   `json:"expenseNumber,omitempty"`
	AmountMinor    string  `json:"amountMinor"`
	ActorID        string  `json:"actorId"`
	ActorName      string  `json:"actorName"`
	Reason         string  `json:"reason"`
	CreatedAt      string  `json:"createdAt"`
	BalanceBefore  *string `json:"balanceBefore,omitempty"`
	BalanceAfter   *string `json:"balanceAfter,omitempty"`
	DrawerJobID    string  `json:"drawerJobId,omitempty"`
	IdempotencyKey string  `json:"-"`
	RequestHash    string  `json:"-"`
}
type ExpenseRequest struct {
	AmountMinor string `json:"amountMinor"`
	Reason      string `json:"reason"`
}
type Repository interface {
	CreditSale(context.Context, Movement, string) error
	Movement(context.Context, string) (Movement, error)
	ExpenseByKey(context.Context, string) (Movement, error)
	CreateExpense(context.Context, *Movement) error
	Balance(context.Context) (string, error)
}
