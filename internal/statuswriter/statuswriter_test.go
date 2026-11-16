package statuswriter

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"slices"
	"testing"
)

// countingWriter records every WriteHeader call so a suppressed duplicate is
// visible (httptest.ResponseRecorder silently ignores it).
type countingWriter struct {
	header   http.Header
	statuses []int
	flushes  int
}

func (c *countingWriter) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}

func (c *countingWriter) WriteHeader(code int) { c.statuses = append(c.statuses, code) }

func (c *countingWriter) Write(p []byte) (int, error) { return len(p), nil }

func (c *countingWriter) Flush() { c.flushes++ }

func TestTracker_DuplicateWriteHeaderSuppressed(t *testing.T) {
	base := &countingWriter{}
	tr := New(base)

	tr.WriteHeader(http.StatusCreated)
	tr.WriteHeader(http.StatusInternalServerError)

	if !slices.Equal(base.statuses, []int{http.StatusCreated}) {
		t.Fatalf("expected only the first status to reach the writer, got %v", base.statuses)
	}
	if !tr.WroteHeader() {
		t.Fatal("expected WroteHeader to report true")
	}
}

func TestTracker_InformationalStatusDoesNotCommit(t *testing.T) {
	base := &countingWriter{}
	tr := New(base)

	tr.WriteHeader(http.StatusEarlyHints)
	tr.WriteHeader(http.StatusOK)

	if !slices.Equal(base.statuses, []int{http.StatusEarlyHints, http.StatusOK}) {
		t.Fatalf("expected 103 and 200 to both reach the writer, got %v", base.statuses)
	}
}

func TestTracker_WriteCommits(t *testing.T) {
	base := &countingWriter{}
	tr := New(base)

	_, _ = tr.Write([]byte("body"))
	tr.WriteHeader(http.StatusOK) // implicit 200 already committed

	if len(base.statuses) != 0 {
		t.Fatalf("a Write already committed the response; got %v", base.statuses)
	}
	if !tr.WroteHeader() {
		t.Fatal("expected WroteHeader to report true after Write")
	}
}

func TestTracker_FlushCommits(t *testing.T) {
	base := &countingWriter{}
	tr := New(base)

	tr.Flush()

	if base.flushes != 1 {
		t.Fatalf("expected the flush to reach the writer, got %d", base.flushes)
	}
	if !tr.WroteHeader() {
		t.Fatal("expected Flush to mark the response started")
	}
}

var errHijacked = errors.New("hijacked")

type hijackPusher struct {
	countingWriter
	pushed    string
	hijacked  bool
	hijackErr error
}

func (w *hijackPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	if w.hijackErr != nil {
		return nil, nil, w.hijackErr
	}
	return nil, nil, nil
}

func (w *hijackPusher) Push(target string, _ *http.PushOptions) error {
	w.pushed = target
	return nil
}

func TestTracker_HijackMarksStarted(t *testing.T) {
	base := &hijackPusher{}
	tr := New(base)

	if _, _, err := tr.Hijack(); err != nil {
		t.Fatalf("Hijack: %v", err)
	}
	if !base.hijacked {
		t.Fatal("Hijack not forwarded")
	}
	if !tr.WroteHeader() {
		t.Fatal("a successful Hijack must mark the response started")
	}
}

func TestTracker_FailedHijackDoesNotMark(t *testing.T) {
	base := &hijackPusher{hijackErr: errHijacked}
	tr := New(base)

	if _, _, err := tr.Hijack(); !errors.Is(err, errHijacked) {
		t.Fatalf("Hijack not forwarded: got %v", err)
	}
	if tr.WroteHeader() {
		t.Fatal("a failed Hijack must not mark the response started")
	}
}

func TestTracker_PushForwarded(t *testing.T) {
	base := &hijackPusher{}
	tr := New(base)

	if err := tr.Push("/next", nil); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if base.pushed != "/next" {
		t.Fatalf("Push not forwarded, got %q", base.pushed)
	}
}

func TestTracker_OptionalInterfacesAndErrNotSupported(t *testing.T) {
	base := &countingWriter{}
	tr := New(base)

	var (
		_ http.Flusher  = tr
		_ http.Hijacker = tr
		_ http.Pusher   = tr
	)

	// countingWriter implements none of the optional interfaces, so the
	// tracker reports ErrNotSupported instead of panicking.
	if _, _, err := tr.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("Hijack on a non-hijackable writer: got %v, want ErrNotSupported", err)
	}
	if err := tr.Push("/", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("Push on a non-pushable writer: got %v, want ErrNotSupported", err)
	}
	if got := tr.Unwrap(); got != http.ResponseWriter(base) {
		t.Fatalf("Unwrap must return the underlying writer, got %T", got)
	}
}
