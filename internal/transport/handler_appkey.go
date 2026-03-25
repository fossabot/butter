package transport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/temikus/butter/internal/appkey"
)

// handleAppKeyCreate vends a new application key.
// POST /v1/app-keys
// Body (optional): {"label": "my-service"}
func (s *Server) handleAppKeyCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
	}
	// Best-effort decode; label is optional.
	_ = json.NewDecoder(r.Body).Decode(&req)

	record, err := s.appKeyStore.Vend(req.Label)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	snap := record.Snapshot()
	_ = json.NewEncoder(w).Encode(snap)
}

// handleAppKeyList returns all provisioned keys and their usage.
// GET /v1/app-keys
func (s *Server) handleAppKeyList(w http.ResponseWriter, _ *http.Request) {
	snapshots := s.appKeyStore.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshots)
}

// handleAppKeyUsage returns usage stats for a specific key.
// GET /v1/app-keys/{key}/usage
func (s *Server) handleAppKeyUsage(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	rec := s.appKeyStore.Lookup(key)
	if rec == nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("app key %q not found", key))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec.Snapshot())
}

// handleUsageAggregate returns aggregate usage across all keys.
// GET /v1/usage
func (s *Server) handleUsageAggregate(w http.ResponseWriter, _ *http.Request) {
	snapshots := s.appKeyStore.List()

	type aggregate struct {
		TotalRequests     int64                           `json:"total_requests"`
		StreamRequests    int64                           `json:"stream_requests"`
		NonStreamRequests int64                           `json:"non_stream_requests"`
		Keys              int                             `json:"keys"`
		Models            map[string]*appkey.ModelSnapshot `json:"models,omitempty"`
	}

	agg := &aggregate{
		Models: make(map[string]*appkey.ModelSnapshot),
	}
	agg.Keys = len(snapshots)

	for _, snap := range snapshots {
		agg.TotalRequests += snap.TotalRequests
		agg.StreamRequests += snap.StreamRequests
		agg.NonStreamRequests += snap.NonStreamRequests
		for model, mu := range snap.Models {
			if existing, ok := agg.Models[model]; ok {
				existing.Requests += mu.Requests
				existing.PromptTokens += mu.PromptTokens
				existing.CompletionTokens += mu.CompletionTokens
			} else {
				agg.Models[model] = &appkey.ModelSnapshot{
					Requests:         mu.Requests,
					PromptTokens:     mu.PromptTokens,
					CompletionTokens: mu.CompletionTokens,
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(agg)
}
