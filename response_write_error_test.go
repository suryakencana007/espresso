package espresso

import (
	"bytes"
	"errors"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// Response write-error fallback (issue #88)
// ============================================
//
// Every built-in IntoResponse commits the status line before any fallible
// operation (JSON encode, body write). When that operation fails — a client
// disconnect, or a payload that cannot be marshaled — the old fallback called
// writeHandlerError, which wrote a second status line. net/http ignores it and
// logs:
//
//	http: superfluous response.WriteHeader call from .../internal/errorenvelope.Write
//
// These tests lock the invariant that a WriteResponse error produces at most
// one WriteHeader: a 500 only when nothing was sent yet, otherwise nothing.

// writeHeaderCounter records how many times WriteHeader was called, so a
// double-write is visible (httptest.ResponseRecorder silently ignores it).
type writeHeaderCounter struct {
	header  http.Header
	status  int
	headerN int
	body    bytes.Buffer
}

func (c *writeHeaderCounter) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}

func (c *writeHeaderCounter) WriteHeader(code int) {
	c.headerN++
	if c.status == 0 {
		c.status = code
	}
}

func (c *writeHeaderCounter) Write(p []byte) (int, error) { return c.body.Write(p) }

// headerThenErrorResponse commits 200, then fails — the client-disconnect and
// marshal-failure shape.
type headerThenErrorResponse struct{}

func (headerThenErrorResponse) WriteResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusOK)
	return errors.New("client went away")
}

// errorBeforeWriteResponse fails without touching the writer — the case a 500
// fallback can still serve.
type errorBeforeWriteResponse struct{}

func (errorBeforeWriteResponse) WriteResponse(http.ResponseWriter) error {
	return errors.New("nothing was written")
}

func TestWriteResponseOrError_NoSecondWriteHeaderAfterResponseStarted(t *testing.T) {
	w := &writeHeaderCounter{}
	writeResponseOrError(w, httptest.NewRequest(http.MethodGet, "/", nil), headerThenErrorResponse{})

	if w.headerN != 1 {
		t.Fatalf("expected exactly one WriteHeader, got %d", w.headerN)
	}
	if w.status != http.StatusOK {
		t.Fatalf("the first status must stand, got %d", w.status)
	}
	if w.body.Len() != 0 {
		t.Fatalf("no fallback body may be written after the response started, got %q", w.body.String())
	}
}

func TestWriteResponseOrError_FallsBackBeforeResponseStarted(t *testing.T) {
	w := &writeHeaderCounter{}
	writeResponseOrError(w, httptest.NewRequest(http.MethodGet, "/", nil), errorBeforeWriteResponse{})

	if w.headerN != 1 {
		t.Fatalf("expected exactly one WriteHeader (the 500), got %d", w.headerN)
	}
	if w.status != http.StatusInternalServerError {
		t.Fatalf("expected a 500 fallback, got %d", w.status)
	}
	if !strings.Contains(w.body.String(), `"code":"INTERNAL"`) {
		t.Fatalf("expected the canonical 500 envelope, got %q", w.body.String())
	}
}

func TestWriteAnyResponseOrError_MarshalFailureDoesNotDoubleWrite(t *testing.T) {
	w := &writeHeaderCounter{}
	// A channel cannot be encoded; JSON[T].WriteResponse commits 200 first.
	writeAnyResponseOrError(w, httptest.NewRequest(http.MethodGet, "/", nil), make(chan int))

	if w.headerN != 1 {
		t.Fatalf("expected exactly one WriteHeader, got %d", w.headerN)
	}
	if w.status != http.StatusOK {
		t.Fatalf("expected the committed 200 to stand, got %d", w.status)
	}
}

// TestRouter_NoSuperfluousWriteHeader asserts the user-visible symptom is gone:
// a real server must not log "superfluous response.WriteHeader call" when a
// handler's response fails to encode after the status line.
func TestRouter_NoSuperfluousWriteHeader(t *testing.T) {
	router := Portafilter()
	router.Get("/bad", func() JSON[chan int] {
		return JSON[chan int]{Data: make(chan int)}
	})

	var logBuf bytes.Buffer
	srv := httptest.NewUnstartedServer(router)
	srv.Config.ErrorLog = stdlog.New(&logBuf, "", 0)
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/bad")
	if err != nil {
		t.Fatalf("GET /bad: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if got := logBuf.String(); strings.Contains(got, "superfluous") {
		t.Fatalf("server logged a superfluous WriteHeader: %q", got)
	}
}
