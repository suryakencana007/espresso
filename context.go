package espresso

import (
	"context"
)

type contextKey string

const (
	userKey    contextKey = "user"
	loggerKey  contextKey = "logger"
	tenantKey  contextKey = "tenant"
	traceIDKey contextKey = "trace_id"
)

// Logger defines an interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Tenant defines an interface for multi-tenant context values.
type Tenant interface {
	GetID() string
	GetName() string
}

// SetUser stores a user value in the context.
func SetUser(ctx context.Context, user interface{}) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetUser retrieves a user value from the context.
func GetUser(ctx context.Context) (interface{}, bool) {
	user := ctx.Value(userKey)
	if user == nil {
		return nil, false
	}
	return user, true
}

// SetLogger stores a logger in the context.
func SetLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// GetLogger retrieves a logger from the context.
func GetLogger(ctx context.Context) (Logger, bool) {
	logger, ok := ctx.Value(loggerKey).(Logger)
	return logger, ok
}

// SetTenant stores a tenant in the context.
func SetTenant(ctx context.Context, tenant Tenant) context.Context {
	return context.WithValue(ctx, tenantKey, tenant)
}

// GetTenant retrieves a tenant from the context.
func GetTenant(ctx context.Context) (Tenant, bool) {
	tenant, ok := ctx.Value(tenantKey).(Tenant)
	return tenant, ok
}

// SetTraceID stores a trace ID in the context.
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves a trace ID from the context.
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(traceIDKey).(string)
	return traceID, ok
}
