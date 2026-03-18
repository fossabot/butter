package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/temikus/butter/internal/config"
	"github.com/temikus/butter/internal/provider"
	"github.com/temikus/butter/internal/provider/openrouter"
	"github.com/temikus/butter/internal/proxy"
	"github.com/temikus/butter/internal/transport"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Build provider registry.
	registry := provider.NewRegistry()

	// Create a shared HTTP client with connection pooling.
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: cfg.Server.WriteTimeout,
	}

	// Register configured providers.
	for name, provCfg := range cfg.Providers {
		switch name {
		case "openrouter":
			registry.Register(openrouter.New(provCfg.BaseURL, httpClient))
		case "openai":
			// OpenAI is OpenAI-compatible, so we reuse the OpenRouter provider
			// with a different base URL. Full OpenAI provider comes in Phase 2.
			registry.Register(&aliasProvider{
				name:     "openai",
				provider: openrouter.New(provCfg.BaseURL, httpClient),
			})
		default:
			logger.Warn("unknown provider, skipping", "provider", name)
		}
	}

	engine := proxy.NewEngine(registry, cfg, logger)
	server := transport.NewServer(&cfg.Server, engine, logger)

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	fmt.Println("butter stopped")
}

// aliasProvider wraps a provider with a different name.
// Used temporarily to support OpenAI via OpenRouter's compatible API.
type aliasProvider struct {
	name     string
	provider provider.Provider
}

func (a *aliasProvider) Name() string                          { return a.name }
func (a *aliasProvider) SupportsOperation(op provider.Operation) bool { return a.provider.SupportsOperation(op) }

func (a *aliasProvider) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return a.provider.ChatCompletion(ctx, req)
}

func (a *aliasProvider) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	return a.provider.ChatCompletionStream(ctx, req)
}

func (a *aliasProvider) Passthrough(ctx context.Context, method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	return a.provider.Passthrough(ctx, method, path, body, headers)
}
