package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/temikus/butter/internal/config"
	"github.com/temikus/butter/internal/provider"
)

type mockProvider struct {
	name     string
	response *provider.ChatResponse
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) SupportsOperation(op provider.Operation) bool { return true }
func (m *mockProvider) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return m.response, nil
}
func (m *mockProvider) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	return nil, nil
}
func (m *mockProvider) Passthrough(ctx context.Context, method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	return nil, nil
}

func newTestEngine(providers ...provider.Provider) *Engine {
	reg := provider.NewRegistry()
	for _, p := range providers {
		reg.Register(p)
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openrouter": {
				Keys: []config.KeyConfig{{Key: "sk-test", Weight: 1}},
			},
			"openai": {
				Keys: []config.KeyConfig{{Key: "sk-openai", Weight: 1}},
			},
		},
		Routing: config.RoutingConfig{
			DefaultProvider: "openrouter",
			Models: map[string]config.ModelRoute{
				"gpt-4o": {Providers: []string{"openai"}, Strategy: "priority"},
			},
		},
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewEngine(reg, cfg, logger)
}

func TestDispatchDefaultProvider(t *testing.T) {
	mock := &mockProvider{
		name: "openrouter",
		response: &provider.ChatResponse{
			RawBody:    []byte(`{"id":"test"}`),
			StatusCode: 200,
		},
	}
	engine := newTestEngine(mock)

	resp, err := engine.Dispatch(context.Background(), []byte(`{"model":"some-model","messages":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDispatchModelRoute(t *testing.T) {
	openrouterMock := &mockProvider{
		name: "openrouter",
		response: &provider.ChatResponse{
			RawBody:    []byte(`{"id":"openrouter"}`),
			StatusCode: 200,
		},
	}
	openaiMock := &mockProvider{
		name: "openai",
		response: &provider.ChatResponse{
			RawBody:    []byte(`{"id":"openai"}`),
			StatusCode: 200,
		},
	}
	engine := newTestEngine(openrouterMock, openaiMock)

	resp, err := engine.Dispatch(context.Background(), []byte(`{"model":"gpt-4o","messages":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.RawBody) != `{"id":"openai"}` {
		t.Errorf("expected openai response, got: %s", resp.RawBody)
	}
}

func TestDispatchExplicitProvider(t *testing.T) {
	openrouterMock := &mockProvider{
		name: "openrouter",
		response: &provider.ChatResponse{
			RawBody:    []byte(`{"id":"openrouter"}`),
			StatusCode: 200,
		},
	}
	openaiMock := &mockProvider{
		name: "openai",
		response: &provider.ChatResponse{
			RawBody:    []byte(`{"id":"openai"}`),
			StatusCode: 200,
		},
	}
	engine := newTestEngine(openrouterMock, openaiMock)

	// Explicitly request openrouter even though gpt-4o routes to openai
	resp, err := engine.Dispatch(context.Background(), []byte(`{"model":"gpt-4o","messages":[],"provider":"openrouter"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.RawBody) != `{"id":"openrouter"}` {
		t.Errorf("expected openrouter response, got: %s", resp.RawBody)
	}
}

func TestDispatchMissingModel(t *testing.T) {
	engine := newTestEngine(&mockProvider{name: "openrouter"})

	_, err := engine.Dispatch(context.Background(), []byte(`{"messages":[]}`))
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestDispatchInvalidJSON(t *testing.T) {
	engine := newTestEngine(&mockProvider{name: "openrouter"})

	_, err := engine.Dispatch(context.Background(), []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDispatchUnknownProvider(t *testing.T) {
	engine := newTestEngine(&mockProvider{name: "openrouter"})

	_, err := engine.Dispatch(context.Background(), []byte(`{"model":"x","provider":"nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
