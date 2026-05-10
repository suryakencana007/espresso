// Package validatehook holds the process-global auto-validation hook used
// by Espresso's built-in extractors. It exists as a leaf package so both
// the root espresso package and the extractor subpackage can depend on it
// without forming an import cycle (the root package's test files import
// extractor, so extractor cannot import root).
//
// Users do not import this package directly. The user-facing API is in
// the root espresso package: SetDefaultValidator, DefaultValidator,
// RunDefaultValidator. Those are thin wrappers around Set / Get / Run
// here.
package validatehook

import "sync/atomic"

// Hook is the signature of an auto-validation function. Extractors call
// the installed Hook (if any) at the end of Extract with a pointer to the
// decoded value.
type Hook func(v any) error

// stored wraps Hook so it can sit inside an atomic.Pointer (Go does not
// permit atomic ops on bare function values).
type stored struct{ fn Hook }

var current atomic.Pointer[stored]

// Set installs (or clears, with nil) the global validation hook. Safe for
// concurrent use; a later call replaces an earlier one.
func Set(fn Hook) {
	if fn == nil {
		current.Store(nil)
		return
	}
	current.Store(&stored{fn: fn})
}

// Get returns the currently-installed hook, or nil if unset. Primarily for
// tests.
func Get() Hook {
	if p := current.Load(); p != nil {
		return p.fn
	}
	return nil
}

// Run executes the installed hook against v. Returns nil if no hook is
// installed; otherwise returns the hook's error. Hot path is a single
// atomic load + branch when nothing is installed.
func Run(v any) error {
	p := current.Load()
	if p == nil {
		return nil
	}
	return p.fn(v)
}
