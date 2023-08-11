package gsr

import (
	"context"
	"io"
	"time"
)

// SimpleCacher interface definition
type SimpleCacher interface {
	// Closer close cache handle
	io.Closer
	// Clear all cache data
	Clear() error

	// Has basic operation
	Has(key string) bool
	Del(key string) error
	Get(key string) any
	Set(key string, val any, ttl time.Duration) error

	// GetMulti multi operation
	GetMulti(keys []string) map[string]any
	SetMulti(values map[string]any, ttl time.Duration) error
	DelMulti(keys []string) error
}

// ContextCacher interface.
type ContextCacher interface {
	SimpleCacher
	// WithContext and clone new cacher
	WithContext(ctx context.Context) ContextCacher
}

// ContextOpCacher interface.
type ContextOpCacher interface {
	SimpleCacher

	// HasWithCtx basic operation
	HasWithCtx(ctx context.Context, key string) bool
	DelWithCtx(ctx context.Context, key string) error
	GetWithCtx(ctx context.Context, key string) any
	SetWithCtx(ctx context.Context, key string, val any, ttl time.Duration) error

	// MGetWithCtx multi keys operation
	MGetWithCtx(ctx context.Context, keys []string) map[string]any
	MSetWithCtx(ctx context.Context, values map[string]any, ttl time.Duration) error
	MDelWithCtx(ctx context.Context, keys []string) error
}

// CodedCacher interface.
type CodedCacher interface {
	SimpleCacher

	// GetAs get and decode cache value to object ptr
	GetAs(key string, ptr any) error
}
