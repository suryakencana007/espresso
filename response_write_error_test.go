package espresso

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
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

// writeHeaderCounter records every WriteHeader call, so a double-write is
// visible (httptest.ResponseRecorder silently ignores it).
type writeHeaderCounter struct {
	header   http.Header
	status   int
	headerN  int
	statuses []int
	flushes  int
	body     bytes.Buffer
}

func (c *writeHeaderCounter) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}

func (c *writeHeaderCounter) WriteHeader(code int) {
	c.headerN++
	c.statuses = append(c.statuses, code)
	if c.status == 0 {
		c.status = code
	}
}

func (c *writeHeaderCounter) Write(p []byte) (int, error) { return c.body.Write(p) }

func (c *writeHeaderCounter) Flush() { c.flushes++ }

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

// earlyHintsThenOKResponse sends an informational response and then the final
// one; net/http allows this and both must reach the writer.
type earlyHintsThenOKResponse struct{}

func (earlyHintsThenOKResponse) WriteResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusEarlyHints)
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestWriteResponseOrError_InformationalStatusDoesNotCommit(t *testing.T) {
	w := &writeHeaderCounter{}
	writeResponseOrError(w, httptest.NewRequest(http.MethodGet, "/", nil), earlyHintsThenOKResponse{})

	want := []int{http.StatusEarlyHints, http.StatusOK}
	if !slices.Equal(w.statuses, want) {
		t.Fatalf("expected the 103 and the final 200 to both reach the writer, got %v", w.statuses)
	}
}

// flushThenErrorResponse flushes (which commits a 200) and then fails.
type flushThenErrorResponse struct{}

func (flushThenErrorResponse) WriteResponse(w http.ResponseWriter) error {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return errors.New("boom after flush")
}

func TestWriteResponseOrError_FlushCommitsResponse(t *testing.T) {
	w := &writeHeaderCounter{}
	writeResponseOrError(w, httptest.NewRequest(http.MethodGet, "/", nil), flushThenErrorResponse{})

	if w.flushes != 1 {
		t.Fatalf("expected the flush to reach the writer, got %d", w.flushes)
	}
	if w.headerN != 0 {
		t.Fatalf("a flush already committed the response; fallback must not WriteHeader, got %v", w.statuses)
	}
	if w.body.Len() != 0 {
		t.Fatalf("no fallback body expected after a flush, got %q", w.body.String())
	}
}

var errHijacked = errors.New("hijacked")

// hijackPusherWriter implements the optional writer interfaces so the tracker's
// forwarding can be observed.
type hijackPusherWriter struct {
	writeHeaderCounter
	pushed string
}

func (w *hijackPusherWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errHijacked
}

func (w *hijackPusherWriter) Push(target string, _ *http.PushOptions) error {
	w.pushed = target
	return nil
}

func TestResponseTracker_ForwardsOptionalInterfaces(t *testing.T) {
	base := &writeHeaderCounter{}
	tracked := &responseTracker{ResponseWriter: base}

	var (
		_ http.Flusher  = tracked
		_ http.Hijacker = tracked
		_ http.Pusher   = tracked
	)

	// The counter implements none of them, so the tracker reports
	// ErrNotSupported instead of panicking.
	if _, _, err := tracked.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("Hijack on a non-hijackable writer: got %v, want ErrNotSupported", err)
	}
	if err := tracked.Push("/", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("Push on a non-pushable writer: got %v, want ErrNotSupported", err)
	}
}

func TestResponseTracker_HijackPushForward(t *testing.T) {
	base := &hijackPusherWriter{}
	tracked := &responseTracker{ResponseWriter: base}

	if _, _, err := tracked.Hijack(); !errors.Is(err, errHijacked) {
		t.Fatalf("Hijack not forwarded: got %v", err)
	}
	if err := tracked.Push("/next", nil); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if base.pushed != "/next" {
		t.Fatalf("Push not forwarded, got %q", base.pushed)
	}
}
