package ratelimit

import (
	"net/http"
	"testing"
	"time"

	"github.com/temikus/butter/internal/plugin"
)

func TestPluginName(t *testing.T) {
	p := New()
	if p.Name() != "ratelimit" {
		t.Fatalf("expected name %q, got %q", "ratelimit", p.Name())
	}
}

func TestPluginClose(t *testing.T) {
	p := New()
	if err := p.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPluginInitDefaults(t *testing.T) {
	p := New()
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init(nil) failed: %v", err)
	}
	if p.rpm != 60 {
		t.Errorf("expected rpm=60, got %d", p.rpm)
	}
	if p.perIP {
		t.Error("expected perIP=false by default")
	}
}

func TestPluginInitCustomConfig(t *testing.T) {
	p := New()
	err := p.Init(map[string]any{
		"requests_per_minute": 100,
		"per_ip":              true,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if p.rpm != 100 {
		t.Errorf("expected rpm=100, got %d", p.rpm)
	}
	if !p.perIP {
		t.Error("expected perIP=true")
	}
}

func TestGlobalRateLimit(t *testing.T) {
	p := New()
	_ = p.Init(map[string]any{"requests_per_minute": 5})

	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	// First 5 requests should pass.
	for i := 0; i < 5; i++ {
		ctx := &plugin.RequestContext{Request: req, Metadata: make(map[string]any)}
		_ = p.PreHTTP(ctx)
		if ctx.ShortCircuit {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	// 6th request should be rate-limited.
	ctx := &plugin.RequestContext{Request: req, Metadata: make(map[string]any)}
	_ = p.PreHTTP(ctx)
	if !ctx.ShortCircuit {
		t.Fatal("6th request should have been rate-limited")
	}
	if ctx.ShortCircuitStatus != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", ctx.ShortCircuitStatus)
	}
	if len(ctx.ShortCircuitBody) == 0 {
		t.Error("expected non-empty short-circuit body")
	}
}

func TestPerIPRateLimit(t *testing.T) {
	p := New()
	_ = p.Init(map[string]any{
		"requests_per_minute": 2,
		"per_ip":              true,
	})

	reqA, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	reqA.RemoteAddr = "10.0.0.1:1234"

	reqB, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	reqB.RemoteAddr = "10.0.0.2:5678"

	// Exhaust client A's quota.
	for i := 0; i < 2; i++ {
		ctx := &plugin.RequestContext{Request: reqA, Metadata: make(map[string]any)}
		_ = p.PreHTTP(ctx)
		if ctx.ShortCircuit {
			t.Fatalf("client A request %d should have been allowed", i+1)
		}
	}

	// Client A should now be rate-limited.
	ctxA := &plugin.RequestContext{Request: reqA, Metadata: make(map[string]any)}
	_ = p.PreHTTP(ctxA)
	if !ctxA.ShortCircuit {
		t.Fatal("client A should be rate-limited")
	}

	// Client B should still be allowed.
	ctxB := &plugin.RequestContext{Request: reqB, Metadata: make(map[string]any)}
	_ = p.PreHTTP(ctxB)
	if ctxB.ShortCircuit {
		t.Fatal("client B should NOT be rate-limited")
	}
}

func TestTokenRefill(t *testing.T) {
	p := New()
	_ = p.Init(map[string]any{"requests_per_minute": 60})

	// Drain the bucket manually.
	now := time.Now()
	p.global = newBucket(60, now)
	p.global.tokens = 0
	p.global.lastFill = now

	// Advance 1 second — should refill 1 token (60/min = 1/sec).
	future := now.Add(1 * time.Second)
	if !p.global.allow(future) {
		t.Fatal("expected token refill to allow request after 1 second")
	}
}

func TestBucketRefillCap(t *testing.T) {
	b := newBucket(10, time.Now())
	// Advance far into the future — tokens should not exceed max.
	future := time.Now().Add(10 * time.Minute)
	b.allow(future) // refills and consumes 1
	if b.tokens > b.max {
		t.Errorf("tokens %f exceeded max %f", b.tokens, b.max)
	}
}

func TestClientIPExtraction(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		xri      string
		remote   string
		expected string
	}{
		{"X-Forwarded-For", "1.2.3.4", "", "5.6.7.8:1234", "1.2.3.4"},
		{"X-Real-IP", "", "2.3.4.5", "5.6.7.8:1234", "2.3.4.5"},
		{"RemoteAddr", "", "", "5.6.7.8:1234", "5.6.7.8"},
		{"RemoteAddr no port", "", "", "5.6.7.8", "5.6.7.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			got := clientIP(req)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestStreamChunkPassthrough(t *testing.T) {
	p := New()
	chunk := []byte("data: {\"test\": true}")
	out, err := p.StreamChunk(nil, chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(chunk) {
		t.Errorf("expected chunk passthrough, got %q", string(out))
	}
}

func TestPostHTTPNoop(t *testing.T) {
	p := New()
	if err := p.PostHTTP(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
