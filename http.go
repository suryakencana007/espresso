package espresso

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/bytedance/sonic"
)

// Memory limits for safe JSON decoding.
// These constants prevent memory exhaustion attacks from large payloads.
const (
	// MaxPayloadSize is the maximum allowed size for request bodies (1MB).
	// Requests exceeding this limit will fail during decode.
	MaxPayloadSize = 1 * 1024 * 1024 // 1MB

	// MaxPoolSize is the maximum buffer size that will be returned to the pool.
	// Buffers larger than this are discarded to prevent memory bloat.
	MaxPoolSize = 64 * 1024 // 64KB
)

// bufferPool is a global pool for byte buffers used during JSON decoding.
// Reusing buffers reduces allocations and GC pressure.
var bufferPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
}

// getBuffer retrieves a buffer from the pool.
// The returned buffer should be returned to the pool using putBuffer.
func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer) //nolint:errcheck // safe: Pool always returns *bytes.Buffer
}

// putBuffer returns a buffer to the pool.
// Buffers larger than MaxPoolSize are discarded to prevent memory bloat.
func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() > MaxPoolSize {
		return // Discard large buffers
	}
	buf.Reset()
	bufferPool.Put(buf)
}

// DecodeSafeJSONLimit safely decodes JSON from an HTTP request body,
// rejecting bodies larger than limit with ErrRequestEntityTooLarge (413).
// Reads through a pooled buffer under an io.LimitReader capped at
// limit+1 so a body of exactly limit bytes decodes successfully; one
// extra byte triggers the over-limit response.
//
// Closes r.Body on return. On a JSON parse error the returned error is
// wrapped with the sonic message for debuggability. On over-limit the
// returned error is an *espresso.Error(413) whose message names the
// exceeded cap.
func DecodeSafeJSONLimit[Req any](r *http.Request, req *Req, limit int64) error {
	defer func() { _ = r.Body.Close() }()

	buf := getBuffer()
	defer putBuffer(buf)

	// limit+1 lets a body of exactly `limit` bytes succeed; one more
	// byte gets us into over-limit territory below.
	n, err := buf.ReadFrom(io.LimitReader(r.Body, limit+1))
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read body: %w", err)
	}
	if n > limit {
		return ErrRequestEntityTooLarge(fmt.Sprintf("body exceeds %d bytes", limit))
	}

	if err := sonic.Unmarshal(buf.Bytes(), req); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}
	return nil
}

// DecodeSafeJSON safely decodes JSON from an HTTP request body with memory protection.
// It uses a pooled buffer with size limiting to prevent memory exhaustion attacks.
//
// Features:
//   - Memory-limited reading (MaxPayloadSize)
//   - Buffer pooling for reduced allocations
//   - Safe against large payload attacks
//
// For a configurable cap, use DecodeSafeJSONLimit directly.
//
// Example:
//
//	var req CreateUserReq
//	if err := DecodeSafeJSON(r, &req); err != nil {
//	    http.Error(w, err.Error(), http.StatusBadRequest)
//	    return
//	}
func DecodeSafeJSON[Req any](r *http.Request, req *Req) error {
	return DecodeSafeJSONLimit(r, req, MaxPayloadSize)
}
