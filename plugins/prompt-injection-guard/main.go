//go:build tinygo || wasip1

// Package main implements the prompt-injection-guard WASM plugin for Butter.
// It scans chat completion request bodies for known prompt injection patterns
// at the pre_http hook, before any provider call.
//
// Build with TinyGo:
//
//	tinygo build -o prompt-injection-guard.wasm -scheduler=none -target=wasi ./plugins/prompt-injection-guard/
//
// Or via the justfile:
//
//	just build-injection-guard
package main

import (
	"encoding/json"
	"strings"

	pdk "github.com/extism/go-pdk"
)

// request mirrors [sdk.Request]. The types are duplicated here to keep
// the plugin self-contained — WASM guest code should not import host packages.
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

type chatBody struct {
	Messages []chatMessage `json:"messages"`
}

const blockBody = `{"error":{"message":"Request blocked: potential prompt injection detected","type":"invalid_request_error","code":"prompt_injection_detected"}}`

// pre_http is called before each request is routed to a provider.
// It scans chat messages for prompt injection patterns and either
// blocks the request (default) or tags it with detection metadata.
//
//export pre_http
func preHTTP() int32 {
	var req request
	if err := json.Unmarshal(pdk.Input(), &req); err != nil {
		pdk.SetError(err)
		return 1
	}

	mode := pdk.GetConfig("mode")
	if mode == "" {
		mode = "block"
	}
	scanRolesRaw := pdk.GetConfig("scan_roles")
	if scanRolesRaw == "" {
		scanRolesRaw = "user,assistant"
	}

	scanAll := false
	roleSet := make(map[string]bool)
	for _, r := range strings.Split(scanRolesRaw, ",") {
		trimmed := strings.TrimSpace(strings.ToLower(r))
		if trimmed == "all" {
			scanAll = true
			break
		}
		roleSet[trimmed] = true
	}

	var body chatBody
	if err := json.Unmarshal(req.Body, &body); err != nil {
		// Not a chat request or malformed — pass through.
		out, _ := json.Marshal(response{})
		pdk.Output(out)
		return 0
	}

	matched, matchedText, matchedCategory := scan(body.Messages, roleSet, scanAll)
	if !matched {
		out, _ := json.Marshal(response{})
		pdk.Output(out)
		return 0
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["prompt_injection_detected"] = true
	meta["matched_pattern"] = matchedText
	meta["matched_category"] = matchedCategory

	switch mode {
	case "log", "tag":
		out, _ := json.Marshal(response{Metadata: meta})
		pdk.Output(out)
	default: // "block" and unknown modes fail-safe to block
		out, _ := json.Marshal(response{
			Metadata:           meta,
			ShortCircuit:       true,
			ShortCircuitStatus: 400,
			ShortCircuitBody:   json.RawMessage(blockBody),
		})
		pdk.Output(out)
	}
	return 0
}

func main() {}
