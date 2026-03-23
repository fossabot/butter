package cache

import (
	"testing"
)

func TestDeriveKeyDeterministic(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"temperature":0}`)
	k1 := DeriveKey("openai", body)
	k2 := DeriveKey("openai", body)
	if k1 != k2 {
		t.Fatalf("same input produced different keys: %s vs %s", k1, k2)
	}
}

func TestDeriveKeyDifferentProvider(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[]}`)
	k1 := DeriveKey("openai", body)
	k2 := DeriveKey("openrouter", body)
	if k1 == k2 {
		t.Fatal("different providers should produce different keys")
	}
}

func TestDeriveKeyIgnoresExtraFields(t *testing.T) {
	body1 := []byte(`{"model":"gpt-4o","messages":[],"stream":false}`)
	body2 := []byte(`{"model":"gpt-4o","messages":[],"stream":false,"user":"test"}`)
	k1 := DeriveKey("openai", body1)
	k2 := DeriveKey("openai", body2)
	// Extra fields like "user" are not part of cache key.
	if k1 != k2 {
		t.Fatalf("extra fields should not affect key: %s vs %s", k1, k2)
	}
}

func TestDeriveKeyWhitespaceNormalized(t *testing.T) {
	body1 := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	body2 := []byte(`{  "model" : "gpt-4o" , "messages" : [ { "role" : "user" , "content" : "hi" } ] }`)
	k1 := DeriveKey("openai", body1)
	k2 := DeriveKey("openai", body2)
	if k1 != k2 {
		t.Fatalf("whitespace differences should not affect key: %s vs %s", k1, k2)
	}
}

func TestIsCacheableNonStreaming(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		expect bool
	}{
		{"temp=0", `{"model":"m","temperature":0}`, true},
		{"temp unset", `{"model":"m"}`, true},
		{"temp=0.7", `{"model":"m","temperature":0.7}`, false},
		{"streaming", `{"model":"m","stream":true}`, false},
		{"streaming temp=0", `{"model":"m","stream":true,"temperature":0}`, false},
		{"invalid json", `not json`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCacheable([]byte(tt.body))
			if got != tt.expect {
				t.Errorf("IsCacheable(%s) = %v, want %v", tt.body, got, tt.expect)
			}
		})
	}
}
