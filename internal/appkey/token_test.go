package appkey

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	key, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(key) != TokenLen {
		t.Errorf("expected len %d, got %d: %q", TokenLen, len(key), key)
	}
	if !strings.HasPrefix(key, Prefix) {
		t.Errorf("expected prefix %q, got %q", Prefix, key)
	}
}

func TestGenerateUnique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := range 100 {
		key, err := Generate()
		if err != nil {
			t.Fatalf("Generate() iteration %d error: %v", i, err)
		}
		if seen[key] {
			t.Fatalf("collision at iteration %d: %q", i, key)
		}
		seen[key] = true
	}
}

func TestIsValid(t *testing.T) {
	key, _ := Generate()
	if !IsValid(key) {
		t.Errorf("generated key %q should be valid", key)
	}
}

func TestIsValid_InvalidCases(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"too short", "btr_abc"},
		{"wrong prefix", "key_7kX9mQ2pL4wR1nB8vC3j"},
		{"too long", "btr_7kX9mQ2pL4wR1nB8vC3jX"},
		{"special chars", "btr_7kX9mQ2pL4wR1nB8vC3!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsValid(tc.key) {
				t.Errorf("IsValid(%q) should be false", tc.key)
			}
		})
	}
}
