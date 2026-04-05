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

// DecodeSafeJSON safely decodes JSON from an HTTP request body with memory protection.
// It uses a pooled buffer with size limiting to prevent memory exhaustion attacks.
//
// Features:
//   - Memory-limited reading (MaxPayloadSize)
//   - Buffer pooling for reduced allocations
//   - Safe against large payload attacks
//
// Example:
//
//	var req CreateUserReq
//	if err := DecodeSafeJSON(r, &req); err != nil {
//	    http.Error(w, err.Error(), http.StatusBadRequest)
//	    return
//	}
func DecodeSafeJSON[Req any](r *http.Request, req *Req) error {
	buf := getBuffer()
	defer putBuffer(buf)

	// Limit reader to prevent memory exhaustion
	limitedReader := io.LimitReader(r.Body, MaxPayloadSize)
	if _, err := buf.ReadFrom(limitedReader); err != nil && err != io.EOF {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if err := sonic.Unmarshal(buf.Bytes(), req); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}
	return nil
}
