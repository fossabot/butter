//go:build tinygo || wasip1

// Package main implements an example Butter WASM plugin using the
// Extism Go PDK. It demonstrates the pre_http hook by tagging each
// request with metadata that is visible to all subsequent hooks.
//
// Build with TinyGo:
//
//	tinygo build -o example-wasm.wasm -scheduler=none -target=wasi ./plugins/example-wasm/
//
// Or via the justfile:
//
//	just build-example-wasm
package main

import (
	"encoding/json"

	pdk "github.com/extism/go-pdk"
)

// request mirrors [sdk.Request]. The types are duplicated here to keep
// the plugin self-contained — WASM guest code should not import host
// packages.
type request struct {
	Hook     string            `json:"hook"`
	Provider string            `json:"provider,omitempty"`
	Model    string            `json:"model,omitempty"`
	Body     json.RawMessage   `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// response mirrors [sdk.Response].
type response struct {
	Body               json.RawMessage `json:"body,omitempty"`
	Metadata           map[string]any  `json:"metadata,omitempty"`
	ShortCircuit       bool            `json:"short_circuit,omitempty"`
	ShortCircuitBody   json.RawMessage `json:"short_circuit_body,omitempty"`
	ShortCircuitStatus int             `json:"short_circuit_status,omitempty"`
}

// pre_http is called before each request is routed to a provider.
// This example tags the request metadata so downstream plugins and
// logging can see that the plugin ran.
//
//export pre_http
func preHTTP() int32 {
	var req request
	if err := json.Unmarshal(pdk.Input(), &req); err != nil {
		pdk.SetError(err)
		return 1
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["tagged_by"] = "example-wasm-plugin"
	meta["plugin_hook"] = "pre_http"

	// Read plugin config (set via wasm_plugins[].config in butter config).
	if tag := pdk.GetConfig("tag"); tag != "" {
		meta["custom_tag"] = tag
	}

	out, err := json.Marshal(response{Metadata: meta})
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Output(out)
	return 0
}

func main() {}
