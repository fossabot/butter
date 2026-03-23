# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
just build                            # Build binary (with commit hash)
just build-release                    # Build with full version info from git
just serve                            # Run with config (auto-loads API keys)
just test                             # Run all tests with race detector
just vet                              # Static analysis
just lint                             # Run golangci-lint
just check                            # Run vet + lint + test
just bench                            # Run benchmarks with allocation reporting
just release-snapshot                 # Test GoReleaser locally (no publish)
just test-one ./internal/proxy/ TestDispatch  # Run a single test
```

## Architecture

Butter is an AI proxy gateway that exposes an OpenAI-compatible API and forwards requests to backend AI providers. The request flow is:

```
Client → transport.Server (HTTP) → proxy.Engine (routing/dispatch) → provider.Registry → Provider impl → upstream API
```

**Key packages:**

- `cmd/butter/` — Entrypoint. Wires config, providers, engine, and HTTP server. Sets up graceful shutdown (SIGINT/SIGTERM).
- `internal/config/` — YAML config with `${ENV_VAR}` substitution via regexp. Applies typed defaults for zero-valued fields.
- `internal/transport/` — HTTP server using Go 1.22+ `ServeMux` pattern routing. Handles streaming detection via `bytes.Contains` (no full JSON parse) and SSE relay with per-chunk flush via `http.Flusher`.
- `internal/proxy/` — Engine resolves provider via: explicit `provider` field in request → model-based route from config → default provider. Selects API key and dispatches.
- `internal/provider/` — `Provider` interface (`ChatCompletion`, `ChatCompletionStream`, `Passthrough`, `SupportsOperation`) + thread-safe `Registry` (RWMutex).
- `internal/provider/openrouter/` — OpenRouter implementation. Line-based SSE parsing with `bufio.Reader`, `sync.Pool` for buffer reuse. Handles `[DONE]` marker.
- `internal/cache/` — Response cache interface + in-memory LRU with TTL. Cache key derived from SHA256(provider + model + messages + params). Only caches non-streaming requests with temperature=0.

**Endpoints:** `POST /v1/chat/completions`, `GET /healthz`

## Design Constraints

- stdlib-only HTTP (no frameworks) — performance target is <50μs proxy overhead
- Single external dependency: `gopkg.in/yaml.v3`
- Streaming uses direct byte relay (no JSON re-serialization)
- Go 1.22+ required for pattern-based ServeMux routing
- No HashiCorp licensed dependencies

## Phased Roadmap

Phase 1 (PoC) and Phase 2 (Multi-Provider + Routing) are complete. Phase 3 (Plugin System) is partially complete (Go plugin interfaces, built-in plugins done; WASM pending). Phase 4 (Caching + Observability) is in progress — response caching is done, OTel traces pending.
