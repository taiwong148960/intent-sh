// Package schemas embeds version-matched schemas used by provider adapters.
package schemas

import _ "embed"

// ProviderResult is the JSON Schema supplied to provider CLIs.
//
//go:embed provider-result.schema.json
var ProviderResult []byte
