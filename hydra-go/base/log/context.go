package log

import "context"

// ContextKey is used as a context key type
type ContextKey string

const (
	// NoSourceKey is used as a context key to indicate that source should not be logged
	NoSourceKey ContextKey = "no_source"
)

// WithNoSource returns a new context with NoSourceKey set to true
func WithNoSource(ctx context.Context) context.Context {
	return context.WithValue(ctx, NoSourceKey, true)
}

// IsNoSource checks if the context has no_source set
func IsNoSource(ctx context.Context) bool {
	noSource, ok := ctx.Value(NoSourceKey).(bool)
	return ok && noSource
}
