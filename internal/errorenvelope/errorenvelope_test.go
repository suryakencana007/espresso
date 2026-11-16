package errorenvelope

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeEnvelope fails the test if the body is not complete, well-formed JSON.
// A truncated body (the pre-fix behavior when Details could not be encoded)
// makes json.Unmarshal fail.
func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) (code, message, requestID string, details map[string]any) {
	t.Helper()
	var got struct {
		Error struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			Details   map[string]any `json:"details"`
			RequestID string         `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("envelope is not valid JSON (truncated?): %v; body=%q", err, rec.Body.String())
	}
	return got.Error.Code, got.Error.Message, got.Error.RequestID, got.Error.Details
}

func TestWrite_ByteIdentical(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusNotFound, Body{Code: "NOT_FOUND", Message: "resource not found"})

	// json.Encoder.Encode appended a newline; the byte-for-byte output must
	// stay the same.
	const want = "{\"error\":{\"code\":\"NOT_FOUND\",\"message\":\"resource not found\"}}\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != ContentType {
		t.Fatalf("content-type = %q, want %q", ct, ContentType)
	}
}

func TestWrite_EncodableDetailsPreserved(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusBadRequest, Body{
		Code:    "BAD_REQUEST",
		Message: "bad",
		Details: map[string]any{"field": "email"},
	})

	code, _, _, details := decodeEnvelope(t, rec)
	if code != "BAD_REQUEST" {
		t.Fatalf("code = %q", code)
	}
	if details["field"] != "email" {
		t.Fatalf("details not preserved: %v", details)
	}
}

func TestWrite_UnencodableDetailStillWellFormed(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusBadRequest, Body{
		Code:      "BAD_REQUEST",
		Message:   "bad",
		RequestID: "req-1",
		Details:   map[string]any{"oops": make(chan int)},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	code, message, requestID, details := decodeEnvelope(t, rec)
	if code != "BAD_REQUEST" || message != "bad" || requestID != "req-1" {
		t.Fatalf("envelope lost fields: code=%q message=%q requestID=%q", code, message, requestID)
	}
	if details != nil {
		t.Fatalf("unencodable details must be dropped, got %v", details)
	}
}

func TestWrite_NoDetailsOmitsKey(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusInternalServerError, Body{Code: "INTERNAL", Message: "boom"})

	if _, _, _, details := decodeEnvelope(t, rec); details != nil {
		t.Fatalf("nil details must be omitted, got %v", details)
	}
	// The key must be absent, not null.
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["error"]["details"]; ok {
		t.Fatalf("expected no \"details\" key, got body %q", rec.Body.String())
	}
}
