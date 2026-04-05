package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suryakencana007/espresso/extractor"
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
