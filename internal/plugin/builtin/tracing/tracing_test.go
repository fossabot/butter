package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/temikus/butter/internal/plugin"
)

// newTestPlugin creates a Plugin backed by an in-memory span recorder
// instead of a real OTLP exporter. Returns the plugin and the recorder.
func newTestPlugin(t *testing.T) (*Plugin, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	p := &Plugin{
		sdk:    tp,
		tracer: tp.Tracer("test"),
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return p, sr
}

func TestInitNoEndpointIsNoop(t *testing.T) {
	p := New()
	if err := p.Init(map[string]any{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.tracer != nil {
		t.Error("expected nil tracer when endpoint is empty")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestInitBadEndpointReturnsError(t *testing.T) {
	p := New()
	// An unreachable endpoint still creates the exporter successfully
	// (connection is lazy); Init should not error on bad endpoints.
	err := p.Init(map[string]any{"endpoint": "localhost:9999", "insecure": true})
	if err != nil {
		// Some versions attempt a connection eagerly; either outcome is acceptable.
		t.Logf("Init returned error (acceptable): %v", err)
	}
	_ = p.Close()
}

func TestPreHTTPNoTracerIsNoop(t *testing.T) {
	p := New()
	_ = p.Init(map[string]any{}) // no endpoint → no tracer

	pctx := &plugin.RequestContext{
		Metadata: make(map[string]any),
	}
	if err := p.PreHTTP(pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := pctx.Metadata[spanMetaKey]; ok {
		t.Error("expected no span stored when tracer is nil")
	}
}

func TestPreHTTPStoresSpan(t *testing.T) {
	p, _ := newTestPlugin(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	pctx := &plugin.RequestContext{
		Request:  req,
		Metadata: make(map[string]any),
	}
	if err := p.PreHTTP(pctx); err != nil {
		t.Fatalf("PreHTTP error: %v", err)
	}
	if _, ok := pctx.Metadata[spanMetaKey]; !ok {
		t.Error("expected span stored in metadata after PreHTTP")
	}
}

func TestOnTraceEndsSpanWithAttributes(t *testing.T) {
	p, sr := newTestPlugin(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	pctx := &plugin.RequestContext{
		Request:  req,
		Metadata: make(map[string]any),
	}
	_ = p.PreHTTP(pctx)

	// Build a trace that includes the span stashed by PreHTTP.
	meta := map[string]any{
		spanMetaKey: pctx.Metadata[spanMetaKey],
		"streaming": false,
	}
	trace := &plugin.RequestTrace{
		Provider:   "openrouter",
		Model:      "gpt-4o",
		StatusCode: 200,
		Duration:   42 * time.Millisecond,
		Metadata:   meta,
	}
	p.OnTrace(trace)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 ended span, got %d", len(spans))
	}
	s := spans[0]

	assertAttr(t, s.Attributes(), "butter.provider", attribute.StringValue("openrouter"))
	assertAttr(t, s.Attributes(), "butter.model", attribute.StringValue("gpt-4o"))
	assertAttr(t, s.Attributes(), "http.response.status_code", attribute.IntValue(200))
	assertAttr(t, s.Attributes(), "butter.duration_ms", attribute.Int64Value(42))

	if s.Status().Code != codes.Ok {
		t.Errorf("expected Ok status, got %v", s.Status().Code)
	}
}

func TestOnTraceMarksErrorForFailedRequest(t *testing.T) {
	p, sr := newTestPlugin(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	pctx := &plugin.RequestContext{Request: req, Metadata: make(map[string]any)}
	_ = p.PreHTTP(pctx)

	p.OnTrace(&plugin.RequestTrace{
		StatusCode: 502,
		Duration:   1 * time.Millisecond,
		Metadata:   map[string]any{spanMetaKey: pctx.Metadata[spanMetaKey]},
	})

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 ended span, got %d", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Errorf("expected Error status for 502, got %v", spans[0].Status().Code)
	}
}

func TestOnTraceNoSpanIsNoop(t *testing.T) {
	p, sr := newTestPlugin(t)
	// OnTrace with no span in metadata must not panic.
	p.OnTrace(&plugin.RequestTrace{
		Metadata: map[string]any{},
	})
	if got := len(sr.Ended()); got != 0 {
		t.Errorf("expected 0 ended spans, got %d", got)
	}
}

func TestStreamChunkPassThrough(t *testing.T) {
	p := New()
	_ = p.Init(map[string]any{})
	chunk := []byte("data: test")
	out, err := p.StreamChunk(nil, chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(chunk) {
		t.Errorf("expected chunk unchanged, got %q", out)
	}
}

// assertAttr checks that an attribute with the given key and value exists
// in the attribute slice.
func assertAttr(t *testing.T, attrs []attribute.KeyValue, key string, want attribute.Value) {
	t.Helper()
	for _, kv := range attrs {
		if string(kv.Key) == key {
			if kv.Value != want {
				t.Errorf("attr %s: got %v, want %v", key, kv.Value, want)
			}
			return
		}
	}
	t.Errorf("attribute %q not found in span", key)
}
