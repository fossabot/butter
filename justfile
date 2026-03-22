# Default recipe
default: build

# Build the binary (with commit hash)
build:
    go build -ldflags "-X github.com/temikus/butter/internal/version.Commit=$(git rev-parse --short HEAD)" \
      -o pkg/bin/butter ./cmd/butter/

# Build with full version info from git
build-release:
    go build -ldflags "-s -w \
      -X github.com/temikus/butter/internal/version.Version=$(git describe --tags --always --dirty) \
      -X github.com/temikus/butter/internal/version.Commit=$(git rev-parse --short HEAD) \
      -X github.com/temikus/butter/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      -o pkg/bin/butter ./cmd/butter/

# Test release locally (no publish)
release-snapshot:
    goreleaser release --snapshot --clean

# Run from source with config (default: config.example.yaml)
serve config="config.example.yaml":
    #!/usr/bin/env bash
    set -euo pipefail

    # Try to load API keys from files if env vars are not set
    if [ -z "${OPENROUTER_API_KEY:-}" ] && [ -f "$HOME/.openrouter/api-key" ]; then
        export OPENROUTER_API_KEY="$(cat "$HOME/.openrouter/api-key")"
        echo "Loaded OPENROUTER_API_KEY from ~/.openrouter/api-key"
    fi
    if [ -z "${OPENAI_API_KEY:-}" ] && [ -f "$HOME/.openai/api-key" ]; then
        export OPENAI_API_KEY="$(cat "$HOME/.openai/api-key")"
        echo "Loaded OPENAI_API_KEY from ~/.openai/api-key"
    fi

    # Bail if no keys found at all
    if [ -z "${OPENROUTER_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ]; then
        echo "Error: No API keys found." >&2
        echo "" >&2
        echo "Set at least one of:" >&2
        echo "  export OPENROUTER_API_KEY=sk-..." >&2
        echo "  export OPENAI_API_KEY=sk-..." >&2
        echo "" >&2
        echo "Or create a key file:" >&2
        echo "  mkdir -p ~/.openrouter && echo 'sk-...' > ~/.openrouter/api-key" >&2
        echo "  mkdir -p ~/.openai && echo 'sk-...' > ~/.openai/api-key" >&2
        exit 1
    fi

    go run ./cmd/butter/ -config {{config}}

# Run all tests (matches CI)
test:
    go test ./... -v -race -count=1

# Run a single test: just test-one ./internal/proxy/ TestDispatch
test-one pkg name:
    go test {{pkg}} -run {{name}} -v -race -count=1

# Lint
lint:
    golangci-lint run

# Static analysis
vet:
    go vet ./...

# Run all checks (vet + lint + tests)
check: vet lint test

# Benchmarks with allocation reporting
bench:
    go test ./... -bench=. -benchmem

# Remove built binary
clean:
    rm -rf pkg/
