// Package contacts reserves the extension contract; it has no HTTP routes or SQL yet.
package contacts

import "context"

type Party struct {
	ID        string
	Name      string
	Customer  bool
	Supplier  bool
	Reference string // Business reference; fiscal validation belongs to a country adapter.
}
type Directory interface {
	Party(context.Context, string) (Party, error)
	CreateParty(context.Context, Party) error
}
