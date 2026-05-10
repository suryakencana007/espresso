package espresso

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// resetValidatorAfter reinstalls SetDefaultValidator(nil) on cleanup so a
// test that installs a hook doesn't leak state to siblings.
func resetValidatorAfter(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetDefaultValidator(nil) })
}

func TestSetDefaultValidator_NilByDefault(t *testing.T) {
	if DefaultValidator() != nil {
		t.Error("DefaultValidator() returned non-nil at startup, want nil")
	}
}

func TestSetDefaultValidator_RoundTrip(t *testing.T) {
	resetValidatorAfter(t)

	stub := func(any) error { return nil }
	SetDefaultValidator(stub)

	if got := DefaultValidator(); got == nil {
		t.Fatal("DefaultValidator() returned nil after Set")
	}
}

func TestSetDefaultValidator_NilClears(t *testing.T) {
	resetValidatorAfter(t)

	SetDefaultValidator(func(any) error { return nil })
	SetDefaultValidator(nil)

	if DefaultValidator() != nil {
		t.Error("DefaultValidator() returned non-nil after clearing")
	}
}

func TestRunDefaultValidator_NilHook(t *testing.T) {
	// Default state — no hook installed.
	if err := RunDefaultValidator(struct{}{}); err != nil {
		t.Errorf("RunDefaultValidator() with no hook = %v, want nil", err)
	}
}

func TestRunDefaultValidator_HookFires(t *testing.T) {
	resetValidatorAfter(t)

	var calls atomic.Int64
	wantErr := errors.New("rejected")
	SetDefaultValidator(func(v any) error {
		calls.Add(1)
		return wantErr
	})

	got := RunDefaultValidator(struct{}{})
	if calls.Load() != 1 {
		t.Errorf("hook calls = %d, want 1", calls.Load())
	}
	if !errors.Is(got, wantErr) {
		t.Errorf("RunDefaultValidator returned %v, want %v", got, wantErr)
	}
}

// TestJSONExtract_AutoValidate exercises the wired-up path on JSON[T].
type autoValidateReq struct {
	Name string `json:"name"`
}

func TestJSONExtract_AutoValidate_Rejects(t *testing.T) {
	resetValidatorAfter(t)

	rejectErr := errors.New("name must not be empty")
	SetDefaultValidator(func(v any) error {
		// v is *JSON[autoValidateReq]; reach inside and look at .Data.
		// In real use users delegate to validator.Struct which understands
		// the structure. Here we keep it simple.
		if d, ok := v.(*autoValidateReq); ok && d.Name == "" {
			return rejectErr
		}
		return nil
	})

	httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":""}`))
	var ext JSON[autoValidateReq]
	err := ext.Extract(httpReq)
	if !errors.Is(err, rejectErr) {
		t.Errorf("Extract returned %v, want rejectErr", err)
	}
}

func TestJSONExtract_AutoValidate_Accepts(t *testing.T) {
	resetValidatorAfter(t)

	SetDefaultValidator(func(v any) error {
		if d, ok := v.(*autoValidateReq); ok && d.Name == "" {
			return errors.New("empty name")
		}
		return nil
	})

	httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
	var ext JSON[autoValidateReq]
	if err := ext.Extract(httpReq); err != nil {
		t.Errorf("Extract returned %v, want nil for valid payload", err)
	}
	if ext.Data.Name != "alice" {
		t.Errorf("Data.Name = %q, want alice", ext.Data.Name)
	}
}

// TestJSONExtract_AutoValidate_RegisteredHandler walks the full pipeline:
// hook installed, handler registered, malformed request returns 400 JSON.
func TestJSONExtract_AutoValidate_RegisteredHandler(t *testing.T) {
	resetValidatorAfter(t)

	SetDefaultValidator(func(v any) error {
		if d, ok := v.(*autoValidateReq); ok && len(d.Name) < 3 {
			return ErrBadRequest("name must be >= 3 chars")
		}
		return nil
	})

	router := Portafilter().Post("/u", Doppio(func(_ context.Context, req *JSON[autoValidateReq]) (Text, error) {
		return Text{Body: req.Data.Name}, nil
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/u", strings.NewReader(`{"name":"ab"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
	// JSON escapes ">" as ">" — match on the prefix and on the code.
	body := rec.Body.String()
	if !strings.Contains(body, "BAD_REQUEST") || !strings.Contains(body, "name must be") {
		t.Errorf("body did not include the validator's BAD_REQUEST + message: %s", body)
	}
}

// TestJSONExtract_NoValidator_ZeroOverhead is a smoke test that the
// nil-default path works (the actual benchmark lives in
// handler_bench_test.go via BenchmarkJSONExtract_NilValidator).
func TestJSONExtract_NoValidator_ZeroOverhead(t *testing.T) {
	if DefaultValidator() != nil {
		t.Fatalf("test prerequisite violated: a previous test left a validator installed")
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	var ext JSON[autoValidateReq]
	if err := ext.Extract(httpReq); err != nil {
		t.Errorf("Extract with nil hook returned %v, want nil", err)
	}
}
