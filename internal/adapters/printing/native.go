// Package printing adapts the server's OS printer APIs. The browser never sends
// raw printer commands or arbitrary device paths.
package printing

type Native struct{}

func New() *Native { return &Native{} }
