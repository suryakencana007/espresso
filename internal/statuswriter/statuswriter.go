// Package statuswriter provides a ResponseWriter wrapper that records whether
// a response has started, so error paths do not write a second status line.
//
// Once a status line is committed, net/http ignores a later WriteHeader and
// logs "superfluous response.WriteHeader call"; there is no way to replace a
// partial response with a 500 envelope. Both the root package's response-write
// fallback and middleware/http's panic recovery need to know whether that point
// has passed, so the tracking lives here as a stdlib-only leaf rather than
// being duplicated in each package.
package statuswriter

import (
	"bufio"
	"net"
	"net/http"
)

// Tracker wraps a ResponseWriter and records whether the status line has been
// sent. Informational responses (1xx, except 101 Switching Protocols) do not
// commit; a Write or Flush does; a successful Hijack takes the connection out
// of the HTTP response lifecycle. The optional writer interfaces are forwarded
// so the wrapper stays transparent to SSE (Flusher), WebSocket (Hijacker), and
// server push (Pusher).
type Tracker struct {
	http.ResponseWriter
	wrote bool
}

// New wraps w. The returned Tracker must be used in place of w so that writes
// are observed.
func New(w http.ResponseWriter) *Tracker {
	return &Tracker{ResponseWriter: w}
}

// WroteHeader reports whether the response has started.
func (t *Tracker) WroteHeader() bool { return t.wrote }

// WriteHeader records the status line and forwards it. Informational
// responses (1xx, except 101 Switching Protocols) are forwarded without
// marking the response started.
func (t *Tracker) WriteHeader(code int) {
	// 1xx informational responses (except 101) do not commit the status line:
	// net/http allows a final WriteHeader after them.
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		t.ResponseWriter.WriteHeader(code)
		return
	}
	if t.wrote {
		return
	}
	t.wrote = true
	t.ResponseWriter.WriteHeader(code)
}

// Write records that the response started (a Write implies a 200) and
// forwards the body.
func (t *Tracker) Write(p []byte) (int, error) {
	// A Write before any WriteHeader implicitly commits a 200.
	t.wrote = true
	return t.ResponseWriter.Write(p)
}

// Flush marks the response started (a flush commits a 200) and forwards to the
// underlying Flusher when it implements one.
func (t *Tracker) Flush() {
	// Flush commits a 200 before it sends the buffered headers/body.
	t.wrote = true
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying Hijacker; on success the connection has
// left the HTTP response lifecycle, so the tracker marks the response started.
func (t *Tracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := t.ResponseWriter.(http.Hijacker); ok {
		conn, rw, err := h.Hijack()
		if err == nil {
			// A hijacked connection has left the HTTP response lifecycle; a
			// later error path must not try to write an envelope onto it.
			t.wrote = true
		}
		return conn, rw, err
	}
	return nil, nil, http.ErrNotSupported
}

// Push forwards to the underlying Pusher when it implements one.
func (t *Tracker) Push(target string, opts *http.PushOptions) error {
	if p, ok := t.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap exposes the wrapped writer to http.ResponseController.
func (t *Tracker) Unwrap() http.ResponseWriter { return t.ResponseWriter }
