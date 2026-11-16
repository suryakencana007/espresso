package httpmiddleware

import (
	"bufio"
	"bytes"
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
// RecoverMiddleware: no second status line (issue #90)
// ============================================
//
// A handler can write (or flush) a response and then panic. The recover path
// used to answer with the canonical 500 envelope regardless, which net/http
// logs as "superfluous response.WriteHeader call" because the status line was
// already sent. recoverWriter tracks whether the response started so the
// envelope is only written when it still can be.

// countingWriter records every WriteHeader call, so a suppressed duplicate is
// visible (httptest.ResponseRecorder silently ignores it).
type countingWriter struct {
	header   http.Header
	headerN  int
	statuses []int
	flushes  int
	body     bytes.Buffer
}

func (c *countingWriter) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}

func (c *countingWriter) WriteHeader(code int) {
	c.headerN++
	c.statuses = append(c.statuses, code)
}

func (c *countingWriter) Write(p []byte) (int, error) { return c.body.Write(p) }

func (c *countingWriter) Flush() { c.flushes++ }

func TestRecoverMiddleware_NoSecondWriteHeaderAfterResponseStarted(t *testing.T) {
	handler := RecoverMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":true`))
		panic("boom after write")
	}))

	w := &countingWriter{}
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.headerN != 1 {
		t.Fatalf("expected exactly one WriteHeader after a recovered panic, got %v", w.statuses)
	}
	if w.statuses[0] != http.StatusOK {
		t.Fatalf("the committed status must stand, got %d", w.statuses[0])
	}
}

func TestRecoverMiddleware_NoEnvelopeAfterFlushThenPanic(t *testing.T) {
	handler := RecoverMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic("boom after flush")
	}))

	w := &countingWriter{}
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.flushes != 1 {
		t.Fatalf("expected the flush to reach the writer, got %d", w.flushes)
	}
	if w.headerN != 0 {
		t.Fatalf("a flush already committed the response; the envelope must be skipped, got %v", w.statuses)
	}
}

func TestRecoverMiddleware_WritesEnvelopeWhenNothingSent(t *testing.T) {
	handler := RecoverMiddleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom before write")
	}))

	w := &countingWriter{}
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if !slices.Equal(w.statuses, []int{http.StatusInternalServerError}) {
		t.Fatalf("expected a single 500, got %v", w.statuses)
	}
	if !strings.Contains(w.body.String(), `"code":"PANIC"`) {
		t.Fatalf("expected the canonical PANIC envelope, got %q", w.body.String())
	}
}

// TestRecoverMiddleware_NoSuperfluousWriteHeaderLog asserts the user-visible
// symptom is gone on a real server.
func TestRecoverMiddleware_NoSuperfluousWriteHeaderLog(t *testing.T) {
	handler := RecoverMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("boom")
	}))

	var logBuf bytes.Buffer
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ErrorLog = stdlog.New(&logBuf, "", 0)
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if got := logBuf.String(); strings.Contains(got, "superfluous") {
		t.Fatalf("server logged a superfluous WriteHeader: %q", got)
	}
}

// hijackableWriter lets a handler take over the connection (the WebSocket
// upgrade shape), so we can prove a panic afterwards writes no envelope.
type hijackableWriter struct {
	countingWriter
}

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

func TestRecoverMiddleware_NoEnvelopeAfterHijackThenPanic(t *testing.T) {
	handler := RecoverMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if h, ok := w.(http.Hijacker); ok {
			_, _, _ = h.Hijack()
		}
		panic("boom after hijack")
	}))

	w := &hijackableWriter{}
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.headerN != 0 {
		t.Fatalf("a hijacked connection has no HTTP response to write; envelope must be skipped, got %v", w.statuses)
	}
}
