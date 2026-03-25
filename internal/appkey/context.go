package appkey

import "context"

type contextKey struct{}

// WithKey returns a derived context carrying the application key.
func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, contextKey{}, key)
}

// FromContext returns the application key stored in ctx, if any.
func FromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(contextKey{}).(string)
	return key, ok && key != ""
}
