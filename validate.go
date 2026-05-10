package espresso

import "github.com/suryakencana007/espresso/v2/internal/validatehook"

// SetDefaultValidator installs the function called by every built-in
// extractor (JSON[T], Query[T], Path[T], Form[T], Header[T], Cookie[T],
// XML[T], Multipart[T], RawBodyWithHeaders[H]) after decoding. When set, a
// non-nil error from the validator propagates as an extraction failure —
// the framework's structured-JSON 400 path handles it, the handler does not
// run.
//
// Pass nil to disable. The default is disabled (nil), so v1.x behavior is
// preserved unless this is called.
//
// Typical wiring against the bundled struct-tag validator:
//
//	import (
//	    "github.com/suryakencana007/espresso/v2"
//	    "github.com/suryakencana007/espresso/v2/validator"
//	)
//
//	func init() { espresso.SetDefaultValidator(validator.Struct) }
//
//	type CreateUserReq struct {
//	    Name  string `json:"name"  validate:"required,min=3,max=50"`
//	    Email string `json:"email" validate:"required,email"`
//	}
//
//	// In handler — validation already ran; req.Data is known good.
//	func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.Text, error) {
//	    return espresso.Text{Body: "created: " + req.Data.Name}, nil
//	}
//
// SetDefaultValidator is concurrent-safe and may be called more than once
// (a later call replaces the earlier hook). The function it installs must
// itself be safe for concurrent use; the framework calls it exactly once
// per successful extraction with a pointer to the decoded value.
//
// Composability with the Validation layer:
//
//   - SetDefaultValidator runs DURING extraction, on the raw decoded
//     payload. It rejects malformed-by-tag inputs before the handler is
//     scheduled.
//   - The Validation[Req] layer (LayerConfig) runs AFTER extraction, on the
//     full request value. It handles cross-field, ctx-dependent, and
//     I/O-bound checks (database lookups, etc.).
//
// They compose; they don't overlap. Use both freely.
func SetDefaultValidator(fn func(v any) error) {
	validatehook.Set(validatehook.Hook(fn))
}

// DefaultValidator returns the currently-installed hook, or nil if unset.
// Primarily for tests; production code rarely needs to read this directly.
func DefaultValidator() func(v any) error {
	if h := validatehook.Get(); h != nil {
		return func(v any) error { return h(v) }
	}
	return nil
}

// RunDefaultValidator runs the configured DefaultValidator (if any) against
// v. Returns nil if no validator is installed; otherwise returns the
// validator's error.
//
// Built-in extractors call this at the end of their Extract methods. Custom
// Extract implementations that want the same auto-validation behavior
// should call this too — typically as the last line of Extract before
// returning nil.
//
// The nil-fast path is a single atomic load + branch, so the hook is
// effectively free when no validator is installed.
func RunDefaultValidator(v any) error {
	return validatehook.Run(v)
}
