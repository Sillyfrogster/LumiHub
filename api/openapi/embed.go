// Package openapi carries the contract this service answers to and the guide
// that explains it, so the running binary can serve both.
package openapi

import _ "embed"

//go:embed openapi.gen.yaml
var Contract []byte

//go:embed protocol.md
var Guide []byte
