package cache

import (
	"testing"
	"time"
)

func TestMemoryGetSet(t *testing.T) {
	c := NewMemory(10)
	c.Set("k1", []byte("v1"), time.Minute)

	got := c.Get("k1")
	if string(got) != "v1" {
		t.Fatalf("expected v1, got %s", got)
	}
}

func TestMemoryMiss(t *testing.T) {
	c := NewMemory(10)
	if got := c.Get("missing"); got != nil {
		t.Fatalf("expected nil, got %s", got)
	}
}

func TestMemoryUpdate(t *testing.T) {
	c := NewMemory(10)
	c.Set("k1", []byte("v1"), time.Minute)
	c.Set("k1", []byte("v2"), time.Minute)

	if got := c.Get("k1"); string(got) != "v2" {
		t.Fatalf("expected v2, got %s", got)
	}
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", c.Len())
	}
}

func TestMemoryEviction(t *testing.T) {
	c := NewMemory(3)
	c.Set("k1", []byte("v1"), time.Minute)
	c.Set("k2", []byte("v2"), time.Minute)
	c.Set("k3", []byte("v3"), time.Minute)
	// k1 is LRU, should be evicted.
	c.Set("k4", []byte("v4"), time.Minute)

	if c.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.Len())
	}
	if got := c.Get("k1"); got != nil {
		t.Fatalf("expected k1 evicted, got %s", got)
	}
	if got := c.Get("k4"); string(got) != "v4" {
		t.Fatalf("expected v4, got %s", got)
	}
}

func TestMemoryEvictionLRUOrder(t *testing.T) {
	c := NewMemory(3)
	c.Set("k1", []byte("v1"), time.Minute)
	c.Set("k2", []byte("v2"), time.Minute)
	c.Set("k3", []byte("v3"), time.Minute)

	// Access k1 to move it to front — k2 is now LRU.
	c.Get("k1")
	c.Set("k4", []byte("v4"), time.Minute)

	if got := c.Get("k2"); got != nil {
		t.Fatal("expected k2 evicted")
	}
	if got := c.Get("k1"); string(got) != "v1" {
		t.Fatalf("expected k1 still present, got %v", got)
	}
}

func TestMemoryTTLExpiry(t *testing.T) {
	c := NewMemory(10)
	now := time.Now()
	c.now = func() time.Time { return now }

	c.Set("k1", []byte("v1"), 100*time.Millisecond)

	// Before expiry.
	if got := c.Get("k1"); string(got) != "v1" {
		t.Fatalf("expected v1, got %s", got)
	}

	// Advance time past TTL.
	c.now = func() time.Time { return now.Add(200 * time.Millisecond) }

	if got := c.Get("k1"); got != nil {
		t.Fatalf("expected nil after expiry, got %s", got)
	}
	// Expired entry should be removed.
	if c.Len() != 0 {
		t.Fatalf("expected 0 entries after expiry, got %d", c.Len())
	}
}

func TestMemoryLen(t *testing.T) {
	c := NewMemory(10)
	if c.Len() != 0 {
		t.Fatalf("expected 0, got %d", c.Len())
	}
	c.Set("a", []byte("1"), time.Minute)
	c.Set("b", []byte("2"), time.Minute)
	if c.Len() != 2 {
		t.Fatalf("expected 2, got %d", c.Len())
	}
}
