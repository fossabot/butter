package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/temikus/butter/internal/plugin"
)

// Plugin implements a token-bucket rate limiter as a TransportPlugin.
// It supports global and per-client-IP rate limiting.
type Plugin struct {
	mu      sync.Mutex
	rpm     int
	perIP   bool
	buckets map[string]*bucket
	global  *bucket
}

type bucket struct {
	tokens    float64
	max       float64
	refillPer float64 // tokens added per nanosecond
	lastFill  time.Time
}

func newBucket(rpm int, now time.Time) *bucket {
	max := float64(rpm)
	return &bucket{
		tokens:    max,
		max:       max,
		refillPer: max / float64(time.Minute),
		lastFill:  now,
	}
}

func (b *bucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.lastFill)
	b.tokens += float64(elapsed) * b.refillPer
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// New creates a rate limiter plugin with sensible defaults.
func New() *Plugin {
	return &Plugin{
		rpm:     60,
		buckets: make(map[string]*bucket),
	}
}

func (p *Plugin) Name() string { return "ratelimit" }

func (p *Plugin) Init(cfg map[string]any) error {
	if cfg == nil {
		return nil
	}
	if v, ok := cfg["requests_per_minute"].(int); ok && v > 0 {
		p.rpm = v
	}
	if v, ok := cfg["per_ip"].(bool); ok {
		p.perIP = v
	}
	// Pre-create global bucket.
	p.global = newBucket(p.rpm, time.Now())
	return nil
}

func (p *Plugin) Close() error { return nil }

// PreHTTP checks the rate limit and short-circuits with 429 if exceeded.
func (p *Plugin) PreHTTP(ctx *plugin.RequestContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	b := p.getBucket(ctx.Request, now)

	if !b.allow(now) {
		ctx.ShortCircuit = true
		ctx.ShortCircuitStatus = http.StatusTooManyRequests
		ctx.ShortCircuitBody = []byte(fmt.Sprintf(
			`{"error":{"message":"rate limit exceeded (%d requests/minute)","type":"rate_limit_error"}}`,
			p.rpm,
		))
	}
	return nil
}

func (p *Plugin) PostHTTP(_ *plugin.RequestContext) error { return nil }

func (p *Plugin) StreamChunk(_ *plugin.RequestContext, chunk []byte) ([]byte, error) {
	return chunk, nil
}

func (p *Plugin) getBucket(r *http.Request, now time.Time) *bucket {
	if !p.perIP {
		if p.global == nil {
			p.global = newBucket(p.rpm, now)
		}
		return p.global
	}

	key := clientIP(r)
	b, ok := p.buckets[key]
	if !ok {
		b = newBucket(p.rpm, now)
		p.buckets[key] = b
	}
	return b
}

// clientIP extracts the client IP from the request, preferring
// X-Forwarded-For, then X-Real-IP, then the remote address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
