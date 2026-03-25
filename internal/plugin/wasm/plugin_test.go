package wasm_test

import (
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/temikus/butter/internal/plugin"
	pluginwasm "github.com/temikus/butter/internal/plugin/wasm"
)

var discardLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestNew(t *testing.T) {
	p := pluginwasm.New("test", "/tmp/nonexistent.wasm", discardLogger)
	if p.Name() != "test" {
		t.Errorf("Name() = %q, want %q", p.Name(), "test")
	}
}

func TestInit_FileNotFound(t *testing.T) {
	p := pluginwasm.New("test", "/tmp/nonexistent_butter_plugin.wasm", discardLogger)
	err := p.Init(nil)
	if err == nil {
		t.Fatal("Init() with non-existent file should return an error")
	}
}

func TestInit_InvalidWASM(t *testing.T) {
	// Write a file with invalid WASM content.
	f, err := os.CreateTemp("", "invalid_*.wasm")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString("not wasm content"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	p := pluginwasm.New("test", f.Name(), discardLogger)
	err = p.Init(nil)
	if err == nil {
		t.Fatal("Init() with invalid WASM should return an error")
	}
}

func TestClose_Uninitialized(t *testing.T) {
	p := pluginwasm.New("test", "/tmp/nonexistent.wasm", discardLogger)
	// Close on an uninitialized plugin must not panic or error.
	if err := p.Close(); err != nil {
		t.Errorf("Close() on uninitialized plugin returned error: %v", err)
	}
}

func TestStreamChunk_Passthrough(t *testing.T) {
	p := pluginwasm.New("test", "/tmp/nonexistent.wasm", discardLogger)
	rc := &plugin.RequestContext{Request: &http.Request{}}
	chunk := []byte("data: {}\n\n")
	got, err := p.StreamChunk(rc, chunk)
	if err != nil {
		t.Fatalf("StreamChunk() error: %v", err)
	}
	if string(got) != string(chunk) {
		t.Errorf("StreamChunk() = %q, want %q", got, chunk)
	}
}

// TestWithExamplePlugin runs a live invocation test against the compiled
// example WASM plugin. It is skipped when the plugin binary is absent,
// which is the case in CI unless `just build-example-wasm` has been run.
func TestWithExamplePlugin(t *testing.T) {
	wasmPath := "../../../plugins/example-wasm/example-wasm.wasm"
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("example-wasm plugin not built; run `just build-example-wasm` to enable this test")
	}

	p := pluginwasm.New("example-wasm", wasmPath, discardLogger)
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	rc := &plugin.RequestContext{
		Request:  &http.Request{Header: http.Header{}},
		Provider: "openai",
		Model:    "gpt-4o",
		Body:     []byte(`{"messages":[]}`),
		Metadata: map[string]any{},
	}

	if err := p.PreHTTP(rc); err != nil {
		t.Fatalf("PreHTTP() error: %v", err)
	}

	if rc.Metadata["tagged_by"] != "example-wasm-plugin" {
		t.Errorf("expected metadata tagged_by=example-wasm-plugin, got: %v", rc.Metadata)
	}
}
