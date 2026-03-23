package cache

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkMemoryGet(b *testing.B) {
	c := NewMemory(10000)
	c.Set("key", []byte("value"), time.Minute)
	b.ResetTimer()
	for b.Loop() {
		c.Get("key")
	}
}

func BenchmarkMemorySet(b *testing.B) {
	c := NewMemory(10000)
	val := []byte("value")
	b.ResetTimer()
	for b.Loop() {
		c.Set("key", val, time.Minute)
	}
}

func BenchmarkMemorySetEviction(b *testing.B) {
	c := NewMemory(100)
	val := []byte("value")
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Set(fmt.Sprintf("key-%d", i), val, time.Minute)
	}
}

func BenchmarkDeriveKey(b *testing.B) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"What is the meaning of life?"}],"temperature":0,"max_tokens":100}`)
	b.ResetTimer()
	for b.Loop() {
		DeriveKey("openai", body)
	}
}

func BenchmarkIsCacheable(b *testing.B) {
	body := []byte(`{"model":"gpt-4o","stream":false,"temperature":0}`)
	b.ResetTimer()
	for b.Loop() {
		IsCacheable(body)
	}
}
