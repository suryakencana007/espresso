// Package bodylimit is a stdlib-only leaf that shares the JSON/raw-body
// size-limit machinery between the root espresso package and the
// extractor package. Extractor is a leaf sibling of root — a direct edge
// extractor → espresso would introduce a new dependency direction the
// module has otherwise avoided — so the shared plumbing lives here
// (mirroring internal/errorenvelope and internal/validatehook).
//
// What lives here:
//
//   - A context key and getter (From) for the per-request body size limit
//     injected by the root's WithJSONBodyLimit router option. Extractors
//     read the limit from the request context via this getter and enforce
//     the cap in their own Read paths.
//   - Middleware, the http.Handler wrapper that injects a limit into the
//     request context. Consumed by the root's Router.WithJSONBodyLimit.
//   - ErrBodyTooLarge, a stdlib-error sentinel returned by extractors
//     when a request body exceeds the configured cap. Root espresso's
//     writeExtractError detects errors.Is(err, ErrBodyTooLarge) and
//     translates to the canonical 413 Payload Too Large JSON envelope.
//   - ReadAllLimited, a helper that reads up to limit+1 bytes and
//     returns ErrBodyTooLarge (via fmt.Errorf %w) when the reader had
//     more to give. Used by every extractor with a body-reading path.
package bodylimit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ctxKey is the private context-key type used to store the body limit.
// Kept unexported so callers cannot inject a limit via context.WithValue
// directly — the only supported entry point is the Middleware helper.
type ctxKey struct{}

// ErrBodyTooLarge is returned by ReadAllLimited (wrapped via fmt.Errorf
// with %w) when a request body exceeds the configured limit. Root
// espresso's writeExtractError matches this sentinel via errors.Is and
// produces a 413 Payload Too Large response.
var ErrBodyTooLarge = errors.New("request body exceeds configured limit")

// From returns the configured body-size limit from ctx, or defaultLimit
// when no limit was injected. Zero or negative defaults are honored as
// given; the caller is responsible for guarding against those upstream
// (Router.WithJSONBodyLimit normalizes zero/negative to the framework
// default before injecting).
func From(ctx context.Context, defaultLimit int64) int64 {
	if v, ok := ctx.Value(ctxKey{}).(int64); ok {
		return v
	}
	return defaultLimit
}

// Middleware returns an http.Handler wrapper that injects limit into the
// request context. Used by Router.WithJSONBodyLimit.
func Middleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{}, limit)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ReadAllLimited reads all bytes from r up to limit, returning
// ErrBodyTooLarge (wrapped via %w so errors.Is matches) if the reader
// would produce more than limit bytes. Implementation reads limit+1
// bytes from an io.LimitReader; if that many were read, the source had
// at least limit+1 bytes and the cap has been exceeded.
//
// The returned []byte is trimmed to at most limit bytes on over-limit
// so callers that log or record the truncated portion don't hold the
// excess byte.
func ReadAllLimited(r io.Reader, limit int64) ([]byte, error) {
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return data, err
	}
	if int64(len(data)) > limit {
		return data[:limit], fmt.Errorf("body exceeds %d bytes: %w", limit, ErrBodyTooLarge)
	}
	return data, nil
}
