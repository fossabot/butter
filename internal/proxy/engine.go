package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/temikus/butter/internal/config"
	"github.com/temikus/butter/internal/provider"
)

// Engine is the core proxy dispatcher. It resolves which provider to use,
// selects an API key, and dispatches the request.
type Engine struct {
	registry *provider.Registry
	config   *config.Config
	keys     map[string][]config.KeyConfig // provider name -> keys
	logger   *slog.Logger
}

func NewEngine(reg *provider.Registry, cfg *config.Config, logger *slog.Logger) *Engine {
	keys := make(map[string][]config.KeyConfig)
	for name, p := range cfg.Providers {
		keys[name] = p.Keys
	}
	return &Engine{
		registry: reg,
		config:   cfg,
		keys:     keys,
		logger:   logger,
	}
}

// Dispatch handles a non-streaming chat completion request.
func (e *Engine) Dispatch(ctx context.Context, rawBody []byte) (*provider.ChatResponse, error) {
	req, providerName, err := e.parseAndRoute(rawBody)
	if err != nil {
		return nil, err
	}

	p, err := e.registry.Get(providerName)
	if err != nil {
		return nil, err
	}

	req.APIKey = e.selectKey(providerName)
	req.RawBody = rawBody

	e.logger.Info("dispatching request",
		"provider", providerName,
		"model", req.Model,
		"stream", false,
	)

	return p.ChatCompletion(ctx, req)
}

// DispatchStream handles a streaming chat completion request.
func (e *Engine) DispatchStream(ctx context.Context, rawBody []byte) (provider.Stream, error) {
	req, providerName, err := e.parseAndRoute(rawBody)
	if err != nil {
		return nil, err
	}

	p, err := e.registry.Get(providerName)
	if err != nil {
		return nil, err
	}

	req.APIKey = e.selectKey(providerName)
	req.RawBody = rawBody

	e.logger.Info("dispatching stream request",
		"provider", providerName,
		"model", req.Model,
		"stream", true,
	)

	return p.ChatCompletionStream(ctx, req)
}

// parseAndRoute extracts the model from the request body and determines
// which provider should handle it.
func (e *Engine) parseAndRoute(rawBody []byte) (*provider.ChatRequest, string, error) {
	// Minimal parse — only extract fields needed for routing.
	var partial struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Provider string `json:"provider"` // Optional explicit provider override
	}
	if err := json.Unmarshal(rawBody, &partial); err != nil {
		return nil, "", fmt.Errorf("invalid request body: %w", err)
	}

	if partial.Model == "" {
		return nil, "", fmt.Errorf("missing required field: model")
	}

	// Determine provider: explicit override > model route > default
	providerName := partial.Provider
	if providerName == "" {
		if route, ok := e.config.Routing.Models[partial.Model]; ok {
			if len(route.Providers) > 0 {
				providerName = route.Providers[0] // Priority strategy for now
			}
		}
	}
	if providerName == "" {
		providerName = e.config.Routing.DefaultProvider
	}
	if providerName == "" {
		return nil, "", fmt.Errorf("no provider configured for model %q", partial.Model)
	}

	req := &provider.ChatRequest{
		Model:   partial.Model,
		Stream:  partial.Stream,
		RawBody: rawBody,
	}

	return req, providerName, nil
}

// selectKey picks an API key for the given provider.
// For Phase 1, this uses simple round-robin; weighted selection comes in Phase 2.
func (e *Engine) selectKey(providerName string) string {
	keys := e.keys[providerName]
	if len(keys) == 0 {
		return ""
	}
	// Simple: return first key. Weighted selection added in Phase 2.
	return keys[0].Key
}
