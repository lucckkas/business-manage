// Package api exposes the versioned OpenAPI contract for the embedded server.
package api

import "embed"

//go:embed openapi.json
var contract embed.FS

func OpenAPI() []byte {
	data, err := contract.ReadFile("openapi.json")
	if err != nil {
		panic(err)
	}
	return data
}
