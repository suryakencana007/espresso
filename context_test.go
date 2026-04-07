package espresso

import (
	"context"
	"testing"
)

type testUserInfo struct {
	ID   string
	Name string
}

type testLogger struct {
	logs []string
}

func (l *testLogger) Debug(msg string, fields ...any) {
	l.logs = append(l.logs, "DEBUG: "+msg)
}

func (l *testLogger) Info(msg string, fields ...any) {
	l.logs = append(l.logs, "INFO: "+msg)
}

func (l *testLogger) Warn(msg string, fields ...any) {
	l.logs = append(l.logs, "WARN: "+msg)
}

func (l *testLogger) Error(msg string, fields ...any) {
	l.logs = append(l.logs, "ERROR: "+msg)
}

type testTenant struct {
	ID   string
	Name string
}

func (t *testTenant) GetID() string {
	return t.ID
}

func (t *testTenant) GetName() string {
	return t.Name
}

func TestSetGetUser(t *testing.T) {
	ctx := context.Background()
	user := &testUserInfo{ID: "123", Name: "Test User"}

	ctx = SetUser(ctx, user)

	retrieved, ok := GetUser(ctx)
	if !ok {
		t.Error("expected user to be found")
	}

	retrievedUser, ok := retrieved.(*testUserInfo)
	if !ok {
		t.Error("expected user to be of type *testUserInfo")
	}

	if retrievedUser.ID != "123" {
		t.Errorf("expected ID '123', got %q", retrievedUser.ID)
	}

	if retrievedUser.Name != "Test User" {
		t.Errorf("expected Name 'Test User', got %q", retrievedUser.Name)
	}
}

func TestGetUser_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetUser(ctx)
	if ok {
		t.Error("expected user not to be found")
	}
}

func TestSetGetLogger(t *testing.T) {
	ctx := context.Background()
	logger := &testLogger{logs: make([]string, 0)}

	ctx = SetLogger(ctx, logger)

	retrieved, ok := GetLogger(ctx)
	if !ok {
		t.Error("expected logger to be found")
	}

	retrievedLogger, ok := retrieved.(*testLogger)
	if !ok {
		t.Error("expected logger to be of type *testLogger")
	}

	retrievedLogger.Info("test message")

	if len(retrievedLogger.logs) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(retrievedLogger.logs))
	}

	if retrievedLogger.logs[0] != "INFO: test message" {
		t.Errorf("expected 'INFO: test message', got %q", retrievedLogger.logs[0])
	}
}

func TestGetLogger_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetLogger(ctx)
	if ok {
		t.Error("expected logger not to be found")
	}
}

func TestSetGetTenant(t *testing.T) {
	ctx := context.Background()
	tenant := &testTenant{ID: "tenant-123", Name: "Acme Corp"}

	ctx = SetTenant(ctx, tenant)

	retrieved, ok := GetTenant(ctx)
	if !ok {
		t.Error("expected tenant to be found")
	}

	retrievedTenant, ok := retrieved.(*testTenant)
	if !ok {
		t.Error("expected tenant to be of type *testTenant")
	}

	if retrievedTenant.GetID() != "tenant-123" {
		t.Errorf("expected ID 'tenant-123', got %q", retrievedTenant.GetID())
	}

	if retrievedTenant.GetName() != "Acme Corp" {
		t.Errorf("expected Name 'Acme Corp', got %q", retrievedTenant.GetName())
	}
}

func TestGetTenant_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetTenant(ctx)
	if ok {
		t.Error("expected tenant not to be found")
	}
}

func TestSetGetTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "trace-abc-123"

	ctx = SetTraceID(ctx, traceID)

	retrieved, ok := GetTraceID(ctx)
	if !ok {
		t.Error("expected trace ID to be found")
	}

	if retrieved != traceID {
		t.Errorf("expected %q, got %q", traceID, retrieved)
	}
}

func TestGetTraceID_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetTraceID(ctx)
	if ok {
		t.Error("expected trace ID not to be found")
	}
}

func TestContextValues_Isolation(t *testing.T) {
	ctx := context.Background()

	user := &testUserInfo{ID: "user-1", Name: "User One"}
	logger := &testLogger{logs: make([]string, 0)}
	tenant := &testTenant{ID: "tenant-1", Name: "Tenant One"}
	traceID := "trace-123"

	ctx = SetUser(ctx, user)
	ctx = SetLogger(ctx, logger)
	ctx = SetTenant(ctx, tenant)
	ctx = SetTraceID(ctx, traceID)

	retrievedUser, ok := GetUser(ctx)
	if !ok {
		t.Error("expected user to be found")
	}
	userVal, ok := retrievedUser.(*testUserInfo)
	if !ok {
		t.Error("expected user to be of type *testUserInfo")
	}
	if userVal.ID != "user-1" {
		t.Error("user data corrupted")
	}

	retrievedLogger, ok := GetLogger(ctx)
	if !ok {
		t.Error("expected logger to be found")
	}
	loggerVal, ok := retrievedLogger.(*testLogger)
	if !ok {
		t.Error("expected logger to be of type *testLogger")
	}
	if loggerVal != logger {
		t.Error("logger reference corrupted")
	}

	retrievedTenant, ok := GetTenant(ctx)
	if !ok {
		t.Error("expected tenant to be found")
	}
	tenantVal, ok := retrievedTenant.(*testTenant)
	if !ok {
		t.Error("expected tenant to be of type *testTenant")
	}
	if tenantVal.ID != "tenant-1" {
		t.Error("tenant data corrupted")
	}

	retrievedTraceID, ok := GetTraceID(ctx)
	if !ok {
		t.Error("expected trace ID to be found")
	}
	if retrievedTraceID != traceID {
		t.Error("trace ID data corrupted")
	}
}

func TestContextValues_Overwrite(t *testing.T) {
	ctx := context.Background()

	user1 := &testUserInfo{ID: "user-1", Name: "User One"}
	user2 := &testUserInfo{ID: "user-2", Name: "User Two"}

	ctx = SetUser(ctx, user1)
	retrieved, ok := GetUser(ctx)
	if !ok {
		t.Error("expected user to be found")
	}
	userVal, ok := retrieved.(*testUserInfo)
	if !ok {
		t.Error("expected user to be of type *testUserInfo")
	}
	if userVal.ID != "user-1" {
		t.Error("expected first user")
	}

	ctx = SetUser(ctx, user2)
	retrieved, ok = GetUser(ctx)
	if !ok {
		t.Error("expected user to be found")
	}
	userVal2, ok := retrieved.(*testUserInfo)
	if !ok {
		t.Error("expected user to be of type *testUserInfo")
	}
	if userVal2.ID != "user-2" {
		t.Error("expected second user to overwrite first")
	}
}
