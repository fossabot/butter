package cache

import "time"

// Cache is the interface for response caching.
type Cache interface {
	// Get retrieves a cached response by key. Returns nil if not found or expired.
	Get(key string) []byte
	// Set stores a response with the given key and TTL.
	Set(key string, value []byte, ttl time.Duration)
	// Len returns the number of entries in the cache.
	Len() int
}
