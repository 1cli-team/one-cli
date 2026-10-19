// Package contract publishes the versioned management HTTP contract.
package contract

import _ "embed"

//go:embed openapi.json
var OpenAPI []byte
