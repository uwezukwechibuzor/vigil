// Package api embeds the OpenAPI specification for use at runtime.
package api

import _ "embed"

// OpenAPISpec is the embedded OpenAPI 3.1 specification YAML.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
