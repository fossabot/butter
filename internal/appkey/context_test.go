package appkey

import (
	"context"
	"testing"
)

func TestWithKeyAndFromContext(t *testing.T) {
	ctx := context.Background()
	key := "btr_testkey00000000000"

	ctx = WithKey(ctx, key)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected key in context")
	}
	if got != key {
		t.Errorf("expected %q, got %q", key, got)
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("expected false for empty context")
	}
}
