package appkey

import (
	"sync"
	"sync/atomic"
	"time"
)

// ModelUsage tracks per-model counters for a single application key.
type ModelUsage struct {
	Requests         atomic.Int64
	PromptTokens     atomic.Int64
	CompletionTokens atomic.Int64
}

// UsageRecord holds all counters for a single application key.
type UsageRecord struct {
	Key       string
	Label     string
	CreatedAt time.Time

	TotalRequests     atomic.Int64
	StreamRequests    atomic.Int64
	NonStreamRequests atomic.Int64

	mu         sync.RWMutex
	modelUsage map[string]*ModelUsage
}

func (u *UsageRecord) getOrCreateModel(model string) *ModelUsage {
	u.mu.RLock()
	mu, ok := u.modelUsage[model]
	u.mu.RUnlock()
	if ok {
		return mu
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if mu, ok = u.modelUsage[model]; !ok {
		mu = &ModelUsage{}
		u.modelUsage[model] = mu
	}
	return mu
}

// Store holds all provisioned application keys and their usage counters.
// All methods are safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	records map[string]*UsageRecord
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{records: make(map[string]*UsageRecord)}
}

// Provision registers a pre-configured key. Idempotent — calling with the
// same key twice is a no-op.
func (s *Store) Provision(key, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[key]; !exists {
		s.records[key] = &UsageRecord{
			Key:        key,
			Label:      label,
			CreatedAt:  time.Now(),
			modelUsage: make(map[string]*ModelUsage),
		}
	}
}

// Vend generates a new key, provisions it in the store, and returns the record.
func (s *Store) Vend(label string) (*UsageRecord, error) {
	key, err := Generate()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	record := &UsageRecord{
		Key:        key,
		Label:      label,
		CreatedAt:  time.Now(),
		modelUsage: make(map[string]*ModelUsage),
	}
	s.records[key] = record
	s.mu.Unlock()
	return record, nil
}

// Lookup returns the UsageRecord for the given key, or nil if unknown.
func (s *Store) Lookup(key string) *UsageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.records[key]
}

// List returns snapshots of all provisioned keys.
func (s *Store) List() []*UsageSnapshot {
	s.mu.RLock()
	records := make([]*UsageRecord, 0, len(s.records))
	for _, r := range s.records {
		records = append(records, r)
	}
	s.mu.RUnlock()

	snapshots := make([]*UsageSnapshot, len(records))
	for i, r := range records {
		snapshots[i] = r.Snapshot()
	}
	return snapshots
}

// RecordRequest increments counters for key. Safe to call concurrently.
// promptTokens and completionTokens may be 0 (e.g. for streaming requests).
func (s *Store) RecordRequest(key, model string, stream bool, promptTokens, completionTokens int64) {
	rec := s.Lookup(key)
	if rec == nil {
		return
	}
	rec.TotalRequests.Add(1)
	if stream {
		rec.StreamRequests.Add(1)
	} else {
		rec.NonStreamRequests.Add(1)
	}
	if model != "" {
		mu := rec.getOrCreateModel(model)
		mu.Requests.Add(1)
		if promptTokens > 0 {
			mu.PromptTokens.Add(promptTokens)
		}
		if completionTokens > 0 {
			mu.CompletionTokens.Add(completionTokens)
		}
	}
}

// UsageSnapshot is a point-in-time, JSON-serializable view of a UsageRecord.
type UsageSnapshot struct {
	Key               string                    `json:"key"`
	Label             string                    `json:"label"`
	CreatedAt         time.Time                 `json:"created_at"`
	TotalRequests     int64                     `json:"total_requests"`
	StreamRequests    int64                     `json:"stream_requests"`
	NonStreamRequests int64                     `json:"non_stream_requests"`
	Models            map[string]*ModelSnapshot `json:"models,omitempty"`
}

// ModelSnapshot is a JSON-serializable view of a ModelUsage.
type ModelSnapshot struct {
	Requests         int64 `json:"requests"`
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
}

// Snapshot returns a consistent point-in-time view of the record.
func (u *UsageRecord) Snapshot() *UsageSnapshot {
	u.mu.RLock()
	models := make(map[string]*ModelSnapshot, len(u.modelUsage))
	for model, mu := range u.modelUsage {
		models[model] = &ModelSnapshot{
			Requests:         mu.Requests.Load(),
			PromptTokens:     mu.PromptTokens.Load(),
			CompletionTokens: mu.CompletionTokens.Load(),
		}
	}
	u.mu.RUnlock()
	return &UsageSnapshot{
		Key:               u.Key,
		Label:             u.Label,
		CreatedAt:         u.CreatedAt,
		TotalRequests:     u.TotalRequests.Load(),
		StreamRequests:    u.StreamRequests.Load(),
		NonStreamRequests: u.NonStreamRequests.Load(),
		Models:            models,
	}
}
