// Package wasm provides the WASM plugin host for Butter, built on the
// Extism Go SDK (https://github.com/extism/go-sdk) which uses wazero
// as its pure-Go WASM runtime.
//
// Each WASMPlugin wraps a pre-compiled Extism plugin and implements
// both [plugin.TransportPlugin] and [plugin.LLMPlugin]. Hook calls are
// safe for concurrent use: a fresh Plugin instance (cheap with
// [extism.CompiledPlugin]) is created per call and closed when done.
package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	extism "github.com/extism/go-sdk"

	"github.com/temikus/butter/internal/plugin"
	"github.com/temikus/butter/plugin/sdk"
)

// Plugin wraps an Extism-compiled WASM module and exposes it as a
// Butter plugin. It implements [plugin.TransportPlugin] and
// [plugin.LLMPlugin]; a WASM plugin may export any subset of the four
// hook functions (pre_http, post_http, pre_llm, post_llm) — unexported
// hooks are silently skipped.
type Plugin struct {
	name     string
	path     string
	compiled *extism.CompiledPlugin
	logger   *slog.Logger
}

// New creates a WASM plugin that will load its module from path.
// name is used as the plugin identifier in logs and config.
// Call [Plugin.Init] to compile and load the WASM module.
func New(name, path string, logger *slog.Logger) *Plugin {
	return &Plugin{
		name:   name,
		path:   path,
		logger: logger,
	}
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string { return p.name }

// Init compiles the WASM module from the path given at construction.
// cfg is forwarded to the plugin via the Extism manifest config
// (key–value string pairs); plugin authors can read these via the
// Extism PDK config API.
func (p *Plugin) Init(cfg map[string]any) error {
	strCfg := make(map[string]string, len(cfg))
	for k, v := range cfg {
		strCfg[k] = fmt.Sprintf("%v", v)
	}

	manifest := extism.Manifest{
		Wasm:   []extism.Wasm{extism.WasmFile{Path: p.path}},
		Config: strCfg,
	}

	compiled, err := extism.NewCompiledPlugin(
		context.Background(),
		manifest,
		extism.PluginConfig{EnableWasi: true},
		nil,
	)
	if err != nil {
		return fmt.Errorf("wasm plugin %q: loading %s: %w", p.name, p.path, err)
	}
	p.compiled = compiled
	p.logger.Info("wasm plugin loaded", "name", p.name, "path", p.path)
	return nil
}

// Close frees the compiled WASM module.
func (p *Plugin) Close() error {
	if p.compiled != nil {
		if err := p.compiled.Close(context.Background()); err != nil {
			return fmt.Errorf("wasm plugin %q close: %w", p.name, err)
		}
		p.compiled = nil
	}
	return nil
}

// PreHTTP implements [plugin.TransportPlugin]. Calls "pre_http" if exported.
func (p *Plugin) PreHTTP(ctx *plugin.RequestContext) error {
	resp, err := p.call("pre_http", ctx, nil)
	if err != nil {
		return err
	}
	if resp != nil {
		applyToRequestContext(ctx, resp)
	}
	return nil
}

// PostHTTP implements [plugin.TransportPlugin]. Calls "post_http" if exported.
func (p *Plugin) PostHTTP(ctx *plugin.RequestContext) error {
	_, err := p.call("post_http", ctx, nil)
	return err
}

// StreamChunk implements [plugin.TransportPlugin]. WASM plugins do not
// participate in per-chunk streaming hooks (instantiating a WASM plugin
// per chunk would add unacceptable latency). Chunks are passed through.
func (p *Plugin) StreamChunk(_ *plugin.RequestContext, chunk []byte) ([]byte, error) {
	return chunk, nil
}

// PreLLM implements [plugin.LLMPlugin]. Calls "pre_llm" if exported.
func (p *Plugin) PreLLM(ctx *plugin.RequestContext) (*plugin.RequestContext, error) {
	resp, err := p.call("pre_llm", ctx, nil)
	if err != nil {
		return ctx, err
	}
	if resp != nil {
		applyToRequestContext(ctx, resp)
		if len(resp.Body) > 0 {
			ctx.Body = resp.Body
		}
	}
	return ctx, nil
}

// PostLLM implements [plugin.LLMPlugin]. Calls "post_llm" if exported.
func (p *Plugin) PostLLM(ctx *plugin.RequestContext, resp *plugin.Response) (*plugin.Response, error) {
	wasmResp, err := p.call("post_llm", ctx, resp.Body)
	if err != nil {
		return resp, err
	}
	if wasmResp != nil && len(wasmResp.Body) > 0 {
		resp.Body = wasmResp.Body
	}
	return resp, nil
}

// call invokes a named hook function on a fresh plugin instance.
// Returns (nil, nil) if the function is not exported by the WASM module.
// body overrides ctx.Body when non-nil (used for post_llm response body).
func (p *Plugin) call(hook string, ctx *plugin.RequestContext, body json.RawMessage) (*sdk.Response, error) {
	inst, err := p.compiled.Instance(context.Background(), extism.PluginInstanceConfig{})
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: create instance: %w", p.name, err)
	}
	defer inst.Close(context.Background()) //nolint:errcheck

	if !inst.FunctionExists(hook) {
		return nil, nil
	}

	reqBody := body
	if reqBody == nil {
		reqBody = ctx.Body
	}

	headers := make(map[string]string, len(ctx.Request.Header))
	for k, vals := range ctx.Request.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	req := sdk.Request{
		Hook:     hook,
		Provider: ctx.Provider,
		Model:    ctx.Model,
		Body:     reqBody,
		Headers:  headers,
		Metadata: ctx.Metadata,
	}

	input, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q %s: marshal request: %w", p.name, hook, err)
	}

	exitCode, output, err := inst.Call(hook, input)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q %s: %w", p.name, hook, err)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("wasm plugin %q %s: non-zero exit code %d", p.name, hook, exitCode)
	}
	if len(output) == 0 {
		return nil, nil
	}

	var resp sdk.Response
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("wasm plugin %q %s: unmarshal response: %w", p.name, hook, err)
	}
	return &resp, nil
}

// applyToRequestContext merges a sdk.Response into a plugin.RequestContext.
func applyToRequestContext(ctx *plugin.RequestContext, resp *sdk.Response) {
	if len(resp.Metadata) > 0 {
		if ctx.Metadata == nil {
			ctx.Metadata = make(map[string]any)
		}
		for k, v := range resp.Metadata {
			ctx.Metadata[k] = v
		}
	}
	if resp.ShortCircuit {
		ctx.ShortCircuit = true
		status := resp.ShortCircuitStatus
		if status == 0 {
			status = http.StatusBadRequest
		}
		ctx.ShortCircuitStatus = status
		ctx.ShortCircuitBody = resp.ShortCircuitBody
	}
}
