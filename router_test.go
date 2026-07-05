package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suryakencana007/espresso/v2/extractor"
)

func TestPortafilter(t *testing.T) {
	router := Portafilter()
	if router.mux == nil {
		t.Error("expected non-nil mux")
	}
}

func TestRouter_Get(t *testing.T) {
	router := Portafilter()
	router.Get("/test", func() Text {
		return Text{Body: "ok"}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got '%s'", rec.Body.String())
	}
}

func TestRouter_Post(t *testing.T) {
	router := Portafilter()
	router.Post("/test", Doppio(func(ctx context.Context, req *JSON[testReq]) (JSON[testRes], error) {
		return JSON[testRes]{Data: testRes{Message: "created"}}, nil
	}))

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRouter_Put(t *testing.T) {
	router := Portafilter()
	router.Put("/test/{id}", Doppio(func(ctx context.Context, req *extractor.Path[testPathReq]) (Status, error) {
		return Status(http.StatusNoContent), nil
	}))

	req := httptest.NewRequest(http.MethodPut, "/test/123", nil)
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRouter_Delete(t *testing.T) {
	router := Portafilter()
	router.Delete("/test/{id}", func() Status {
		return Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodDelete, "/test/123", nil)
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRouter_Patch(t *testing.T) {
	router := Portafilter()
	router.Patch("/test", func() Text {
		return Text{Body: "patched"}
	})

	req := httptest.NewRequest(http.MethodPatch, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRouter_Options(t *testing.T) {
	router := Portafilter()
	router.Options("/test", func() Status {
		return Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRouter_Head(t *testing.T) {
	router := Portafilter()
	router.Head("/test", func() Status {
		return Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRouter_ChainPattern(t *testing.T) {
	callCount := 0
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			next.ServeHTTP(w, r)
		})
	}

	router := Portafilter().
		Use(middleware).
		Get("/test", func() Text {
			return Text{Body: "ok"}
		}).
		Post("/test", func() Status {
			return Status(http.StatusCreated)
		})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if callCount != 1 {
		t.Errorf("expected middleware to be called 1 time, got %d", callCount)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if callCount != 2 {
		t.Errorf("expected middleware to be called 2 times, got %d", callCount)
	}
}

func TestRouter_MultipleUse(t *testing.T) {
	order := []string{}

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}

	router := Portafilter().
		Use(mw1).
		Use(mw2).
		Get("/test", func() Text {
			order = append(order, "handler")
			return Text{Body: "ok"}
		})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	expected := []string{"mw1", "mw2", "handler"}
	if len(order) != len(expected) {
		t.Errorf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if i >= len(order) || order[i] != v {
			t.Errorf("expected order[%d] = '%s', got '%s'", i, v, order[i])
		}
	}
}

func TestRouter_MiddlewareOrder(t *testing.T) {
	order := []string{}

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	router := Portafilter().
		Use(mw1).
		Use(mw2).
		Get("/test", func() Text {
			order = append(order, "handler")
			return Text{Body: "ok"}
		})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Errorf("expected %d calls, got %d", len(expected), len(order))
	}
}

func TestRouter_NotFound(t *testing.T) {
	router := Portafilter()
	router.Get("/exists", func() Text {
		return Text{Body: "ok"}
	})

	req := httptest.NewRequest(http.MethodGet, "/notexists", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	router := Portafilter()
	router.Get("/test", func() Text {
		return Text{Body: "ok"}
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Go's ServeMux returns 405 Method Not Allowed for existing path but wrong method
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRouter_PathParams(t *testing.T) {
	router := Portafilter()
	router.Get("/users/{id}", Doppio(func(ctx context.Context, req *extractor.Path[testPathReq]) (JSON[testRes], error) {
		return JSON[testRes]{Data: testRes{Message: "user"}}, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRouter_Handle(t *testing.T) {
	router := Portafilter()
	handler := router.Handle(func() Text {
		return Text{Body: "handled"}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRouter_ReturnsPointer(t *testing.T) {
	router := Portafilter()
	result := router.Get("/test", func() Text {
		return Text{Body: "ok"}
	})

	if result == nil {
		t.Error("expected router.Get to return non-nil router pointer")
	}

	result2 := result.Post("/test2", func() Text {
		return Text{Body: "ok"}
	})

	if result2 == nil {
		t.Error("expected router.Post to return non-nil router pointer")
	}
}

type testPathReq struct {
	ID string `path:"id"`
}

// TestRouter_WithJSONBodyLimit_Rejects locks the router-level option.
// A router with WithJSONBodyLimit(N) rejects bodies larger than N with
// a 413 canonical envelope, before the handler is invoked. Pre-fix,
// WithJSONBodyLimit did not exist, so the test could not be written —
// and the extractor decoded arbitrarily large bodies.
func TestRouter_WithJSONBodyLimit_Rejects(t *testing.T) {
	limit := int64(32)

	var handlerCalled bool
	router := Portafilter().
		WithJSONBodyLimit(limit)
	router.Post("/echo", Doppio(func(ctx context.Context, req *JSON[map[string]string]) (JSON[map[string]string], error) {
		handlerCalled = true
		return JSON[map[string]string]{Data: req.Data}, nil
	}))

	body := strings.Repeat("x", int(limit)+1) // over cap
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler must not be invoked when body exceeds limit")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"PAYLOAD_TOO_LARGE"`) {
		t.Errorf("expected canonical envelope with code PAYLOAD_TOO_LARGE, got body: %s", rec.Body.String())
	}
}

// TestRouter_WithJSONBodyLimit_Accepts locks the boundary: a valid
// JSON body at exactly the limit succeeds and the handler runs.
func TestRouter_WithJSONBodyLimit_Accepts(t *testing.T) {
	body := `{"name":"ok"}`
	limit := int64(len(body))

	var got string
	router := Portafilter().WithJSONBodyLimit(limit)
	router.Post("/echo", Doppio(func(ctx context.Context, req *JSON[struct {
		Name string `json:"name"`
	}]) (JSON[struct{}], error) {
		got = req.Data.Name
		return JSON[struct{}]{}, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got != "ok" {
		t.Errorf("handler saw Name=%q, want ok", got)
	}
}

// TestRouter_WithJSONBodyLimit_NonPositiveDefaults locks that
// misconfiguration (limit <= 0) falls back to MaxPayloadSize rather
// than uncapping the router.
func TestRouter_WithJSONBodyLimit_NonPositiveDefaults(t *testing.T) {
	// A router built with limit=0 should still reject bodies over
	// MaxPayloadSize (1 MB) — proving the option normalized 0 to the
	// default rather than storing 0 (which would mean "no cap").
	router := Portafilter().WithJSONBodyLimit(0)
	router.Post("/echo", Doppio(func(ctx context.Context, req *JSON[map[string]string]) (JSON[struct{}], error) {
		return JSON[struct{}]{}, nil
	}))

	body := strings.Repeat("x", int(MaxPayloadSize)+1)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 (0 limit → MaxPayloadSize default), got %d", rec.Code)
	}
}
