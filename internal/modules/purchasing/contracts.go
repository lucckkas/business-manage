// Package purchasing defines the next commercial module, without a runtime stub.
package purchasing

import "context"

type Line struct{ ProductID, Quantity, UnitCostMinor string }
type Order struct {
	ID, SupplierID, State, CreatedAt string
	Lines                            []Line
}

// Receiving must coordinate purchasing, inventory and audit in a single unit of work.
// A later payable must participate in that same transaction when credit purchases exist.
type Receiving interface {
	ReceivePurchase(context.Context, string, string) (Order, error)
}
