package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/temikus/butter/internal/provider"
)

func TestProviderName(t *testing.T) {
	p := New("", nil)
	if p.Name() != "openrouter" {
		t.Errorf("expected openrouter, got %s", p.Name())
	}
}

func TestDefaultBaseURL(t *testing.T) {
	p := New("", nil)
	if p.Name() != "openrouter" {
		t.Errorf("expected openrouter, got %s", p.Name())
	}
}

func TestCustomBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"resp-1"}`)
	}))
	defer server.Close()

	p := New(server.URL, nil)
	resp, err := p.ChatCompletion(context.Background(), &provider.ChatRequest{
		Model:   "test",
		RawBody: []byte(`{"model":"test"}`),
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
