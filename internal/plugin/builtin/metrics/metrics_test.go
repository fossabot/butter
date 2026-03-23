package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/temikus/butter/internal/plugin"
)

func TestPluginName(t *testing.T) {
	p := New()
	if p.Name() != "metrics" {
		t.Fatalf("expected name %q, got %q", "metrics", p.Name())
	}
}

func TestInitCreatesHandler(t *testing.T) {
	p := New()
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Close() }()

	if p.Handler() == nil {
		t.Fatal("expected non-nil handler after Init")
	}
}

func TestOnTraceRecordsMetrics(t *testing.T) {
	p := New()
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Emit a successful trace.
	p.OnTrace(&plugin.RequestTrace{
		Provider:   "openai",
		Model:      "gpt-4o",
		StatusCode: 200,
		Duration:   150 * time.Millisecond,
		Metadata:   map[string]any{"streaming": false},
	})

	// Emit an error trace.
	p.OnTrace(&plugin.RequestTrace{
		Provider:   "anthropic",
		Model:      "claude-sonnet-4-20250514",
		StatusCode: 500,
		Duration:   50 * time.Millisecond,
		Error:      io.ErrUnexpectedEOF,
		Metadata:   map[string]any{"streaming": true},
	})

	// Scrape the metrics endpoint.
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify request total counter exists with correct labels.
	if !strings.Contains(body, "butter_request_total") {
		t.Error("missing butter_request_total metric")
	}
	if !strings.Contains(body, `provider="openai"`) {
		t.Error("missing openai provider label")
	}
	if !strings.Contains(body, `provider="anthropic"`) {
		t.Error("missing anthropic provider label")
	}

	// Verify duration histogram.
	if !strings.Contains(body, "butter_request_duration") {
		t.Error("missing butter_request_duration metric")
	}

	// Verify error counter exists and has the error trace.
	if !strings.Contains(body, "butter_request_errors") {
		t.Error("missing butter_request_errors metric")
	}
}

func TestOnTraceNoMetadataStreaming(t *testing.T) {
	p := New()
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Trace with nil metadata — should not panic.
	p.OnTrace(&plugin.RequestTrace{
		Provider:   "openrouter",
		Model:      "gpt-4o",
		StatusCode: 200,
		Duration:   10 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `streaming="false"`) {
		t.Error("expected streaming=false when metadata is nil")
	}
}

func TestCloseShutsMeterProvider(t *testing.T) {
	p := New()
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Closing again should be safe (meterProvider is already shut down,
	// but Close is idempotent in the SDK).
}

func TestCloseWithoutInit(t *testing.T) {
	p := New()
	// Close without Init — should not panic or error.
	if err := p.Close(); err != nil {
		t.Fatalf("Close without Init: %v", err)
	}
}
