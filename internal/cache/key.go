package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// DeriveKey computes a deterministic cache key from the request body.
// It normalizes the JSON by extracting and sorting relevant fields
// (model, messages, temperature, top_p, max_tokens, stop) to ensure
// semantically identical requests produce the same key.
func DeriveKey(provider string, rawBody []byte) string {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		// Fallback: hash the raw body as-is.
		return hashBytes(provider, rawBody)
	}

	// Extract cacheable fields in sorted order for determinism.
	cacheFields := []string{"max_tokens", "messages", "model", "stop", "temperature", "top_p"}
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte{0}) // separator

	for _, field := range cacheFields {
		if v, ok := parsed[field]; ok {
			h.Write([]byte(field))
			h.Write([]byte{0})
			// Compact the JSON to normalize whitespace.
			compact, err := compactJSON(v)
			if err != nil {
				h.Write(v)
			} else {
				h.Write(compact)
			}
			h.Write([]byte{0})
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// IsCacheable checks whether a request should be cached.
// Only non-streaming requests with temperature=0 (or unset) are cached.
func IsCacheable(rawBody []byte) bool {
	var partial struct {
		Stream      bool     `json:"stream"`
		Temperature *float64 `json:"temperature"`
	}
	if err := json.Unmarshal(rawBody, &partial); err != nil {
		return false
	}
	if partial.Stream {
		return false
	}
	// Cache when temperature is 0 or unset (defaults to deterministic).
	if partial.Temperature != nil && *partial.Temperature != 0 {
		return false
	}
	return true
}

func hashBytes(provider string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte{0})
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func compactJSON(data json.RawMessage) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	v = sortKeys(v)
	return json.Marshal(v)
}

// sortKeys recursively sorts map keys for deterministic serialization.
func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sorted := make(map[string]any, len(val))
		for _, k := range keys {
			sorted[k] = sortKeys(val[k])
		}
		return sorted
	case []any:
		for i, item := range val {
			val[i] = sortKeys(item)
		}
		return val
	default:
		return v
	}
}
