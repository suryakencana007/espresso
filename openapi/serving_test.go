package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// unmarshalableExample is a JSON-hostile value (a channel) used to force
// json.MarshalIndent to fail inside the spec handler, exercising the D1 failure
// path. This mirrors the verify-scope probe.
func unmarshalableExample() any { return make(chan int) }

// TestOpenAPIHandler_MarshalFailure_Envelope drives Handler() down the marshal
// failure path (D1) and asserts the canonical JSON error envelope, not the old
// text/plain http.Error body.
func TestOpenAPIHandler_MarshalFailure_Envelope(t *testing.T) {
	g := New("Test API", "1.0.0")
	// A channel-typed Example value cannot be marshaled by encoding/json, so
	// ToJSON() fails and the handler must emit the envelope.
	g.AddPath(http.MethodGet, "/boom", Operation{
		Summary: "boom",
		Responses: map[string]Response{
			"200": {
				Description: "bad",
				Content: map[string]MediaType{
					"application/json": {Example: unmarshalableExample()},
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
	if strings.Contains(ct, "text/plain") {
		t.Fatalf("failure path regressed to text/plain: %q", ct)
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not the canonical envelope JSON: %v (body=%q)", err, rec.Body.String())
	}
	if env.Error.Code != "INTERNAL" {
		t.Errorf("expected error code INTERNAL, got %q", env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Error("expected a non-empty error message")
	}
}

// TestOpenAPIHandler_SpecBytesStable asserts two successive GETs return
// byte-identical bodies — the cache returns the same marshaled slice (D7).
func TestOpenAPIHandler_SpecBytesStable(t *testing.T) {
	g := New("Test API", "1.0.0")
	g.AddPath(http.MethodGet, "/users", Operation{Summary: "Get users"})
	h := g.Handler()

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("expected 200/200, got %d/%d", rec1.Code, rec2.Code)
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Errorf("spec bodies differ between requests:\n#1=%q\n#2=%q", rec1.Body.String(), rec2.Body.String())
	}
}

// TestOpenAPIHandler_SpecServedFromCache asserts the spec is marshaled exactly
// once across many requests (D7): the first request caches the bytes and every
// subsequent request returns the same backing slice.
func TestOpenAPIHandler_SpecServedFromCache(t *testing.T) {
	g := New("Test API", "1.0.0")
	g.AddPath(http.MethodGet, "/users", Operation{Summary: "Get users"})

	// cachedJSON must return the same slice header (same backing array) on
	// repeated calls without a mutation — proof the marshal happened once.
	first, err := g.cachedJSON()
	if err != nil {
		t.Fatalf("cachedJSON() error = %v", err)
	}
	for i := 0; i < 5; i++ {
		next, err := g.cachedJSON()
		if err != nil {
			t.Fatalf("cachedJSON() call %d error = %v", i, err)
		}
		if &first[0] != &next[0] {
			t.Fatalf("cachedJSON() re-marshaled on call %d (different backing array) — not cached", i)
		}
	}

	// A mutation must invalidate the cache so the next marshal reflects it and
	// returns a fresh slice (not stale bytes).
	g.AddPath(http.MethodGet, "/new", Operation{Summary: "New"})
	after, err := g.cachedJSON()
	if err != nil {
		t.Fatalf("cachedJSON() after mutation error = %v", err)
	}
	if &first[0] == &after[0] {
		t.Fatal("cache was not invalidated after AddPath: served stale bytes")
	}
	if !strings.Contains(string(after), "/new") {
		t.Error("post-mutation spec does not reflect the new path — cache stale")
	}
}

// TestOpenAPIHandler_ConcurrentServe exercises the cache under concurrent serving
// to confirm it is -race clean (the handler serves concurrently in production).
func TestOpenAPIHandler_ConcurrentServe(t *testing.T) {
	g := New("Test API", "1.0.0")
	g.AddPath(http.MethodGet, "/users", Operation{Summary: "Get users"})
	h := g.Handler()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent serve got status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
}

// TestScalarHTML_VersionPinned asserts the served Scalar HTML references a pinned
// @scalar/api-reference@<version> and not the unpinned bare package path (D10).
func TestScalarHTML_VersionPinned(t *testing.T) {
	rec := httptest.NewRecorder()
	ScalarUIHandler("/openapi.json").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	body := rec.Body.String()

	pinned := "@scalar/api-reference@" + ScalarVersion
	if !strings.Contains(body, pinned) {
		t.Errorf("expected pinned Scalar bundle %q in served HTML, got:\n%s", pinned, body)
	}

	// The bare, unpinned path must not appear (it would resolve to latest).
	if strings.Contains(body, `@scalar/api-reference"`) {
		t.Error("served HTML still references the unpinned @scalar/api-reference bundle (no @version)")
	}
	if strings.Contains(body, "SCALAR_VERSION_PLACEHOLDER") {
		t.Error("version placeholder was not substituted in the served HTML")
	}
}
