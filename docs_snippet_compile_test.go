package espresso

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ============================================
// Docs Snippet-Compile Check (v2.3 task-04)
// ============================================
//
// Guards against published Go programs in docs/ that do not compile — the
// "trustworthy artifacts" emphasis of v2.3 applied to the docs. The trigger
// was the espresso.X[ -> extractor.X[ extractor sweep: a reader who copied a
// doc snippet verbatim got `undefined: espresso.Path`. This check extracts the
// self-contained ```go fences from docs/ and `go build`s each one so that class
// of error cannot silently return.
//
// SCOPE — self-contained fences that exercise the espresso module. The rule has
// three clauses, ALL required:
//
//  1. The fence's first meaningful line is `package main` — a runnable program,
//     not a library fragment (`package foo`) or a bare signature/body.
//  2. Every import resolves offline within this module — stdlib (verified
//     against `go list std`, not a dotted-host heuristic) or
//     github.com/suryakencana007/espresso/v2/.... Fences importing fictional
//     helpers (myapp/..., your-app/...) or third-party deps (redis, golang-jwt,
//     …) are skipped: they cannot be `go build`-ed hermetically.
//  3. The fence references the espresso module — either by importing one of its
//     packages OR by *using* an `espresso.`/`extractor.` qualified identifier in
//     the body. This is the v2.3 task-04 hardening (PR #51 follow-up): the
//     original gate keyed on the *extractor import* alone, so a program that
//     USED extractor.X[T] but forgot to `import ".../extractor"` was silently
//     skipped — exactly the missing-import class the guard is meant to catch.
//     Detecting espresso/extractor by usage (not just by import) puts those
//     programs back in scope, so a dropped import now fails to compile and trips
//     CI. Illustrative package-main snippets that only reach fictional/third-
//     party packages remain out of scope by clause 2.
//
// The build runs inside a temp directory created UNDER the module root so the
// snippet is part of this module: `go build` then resolves espresso/extractor/
// openapi from the local go.mod and module cache. It is hermetic — no network,
// no go.mod synthesis — because the fence's imports are already module deps.

const modulePath = "github.com/suryakencana007/espresso/v2"

func TestDocsSnippetsCompile(t *testing.T) {
	docsRoot := "docs"
	if _, err := os.Stat(docsRoot); err != nil {
		t.Skipf("docs/ not present (%v); skipping snippet-compile check", err)
	}

	stdlib := stdlibPackages(t)

	// All self-contained snippets share one temp dir under the module root, each
	// in its own package subdir, so they resolve module deps offline.
	buildRoot, err := os.MkdirTemp(".", "docsnippet-")
	if err != nil {
		t.Fatalf("creating temp build root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildRoot) })

	var built int
	walkErr := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".vitepress" || name == "node_modules" || name == "dist" || name == "public" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // path comes from a trusted docs/ walk
		if readErr != nil {
			return readErr
		}
		rel := filepath.ToSlash(path)
		for i, fence := range goFences(string(content)) {
			if !isSelfContained(fence, stdlib) {
				continue
			}
			built++
			buildSnippet(t, buildRoot, rel, i, fence)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking docs/: %v", walkErr)
	}
	if built == 0 {
		t.Fatal("no self-contained Go snippets found in docs/; the detector is likely broken")
	}
	t.Logf("compiled %d self-contained docs snippet(s)", built)
}

// stdlibPackages returns the set of standard-library import paths, queried once
// from the toolchain so import classification is exact rather than heuristic
// (a hyphenated fictional path like "your-app/..." has no dot in its first
// segment and would otherwise be mistaken for stdlib).
func stdlibPackages(t *testing.T) map[string]bool {
	t.Helper()
	out, err := exec.Command("go", "list", "std").Output()
	if err != nil {
		t.Fatalf("listing stdlib packages: %v", err)
	}
	set := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			set[p] = true
		}
	}
	return set
}

// goFences returns the body of every ```go / ```golang fenced block in content.
func goFences(content string) []string {
	var fences []string
	lines := strings.Split(content, "\n")
	inFence := false
	var cur []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				if lang == "go" || lang == "golang" {
					inFence = true
					cur = nil
				}
			} else {
				inFence = false
				fences = append(fences, strings.Join(cur, "\n"))
			}
			continue
		}
		if inFence {
			cur = append(cur, line)
		}
	}
	return fences
}

// espressoUsageRe matches a qualified use of the espresso or extractor package
// (e.g. `espresso.Portafilter`, `extractor.Query[`). The leading boundary class
// keeps it from matching `myespresso.X` or a field selector `x.extractor.Y`.
var espressoUsageRe = regexp.MustCompile(`(^|[^\w.])(espresso|extractor)\.`)

// isSelfContained reports whether a fence is an in-scope espresso-module program
// per the three-clause rule in the package doc: package-main, all imports
// resolvable offline within this module, and a reference to the espresso module
// (by import or by qualified usage of espresso./extractor.).
func isSelfContained(fence string, stdlib map[string]bool) bool {
	if !startsWithPackageMain(fence) {
		return false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "snippet.go", fence, parser.ImportsOnly)
	if err != nil {
		return false
	}
	importsModule := false
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		if !importResolvable(p, stdlib) {
			return false
		}
		if p == modulePath || strings.HasPrefix(p, modulePath+"/") {
			importsModule = true
		}
	}
	// Clause 3: reference the espresso module by import OR by usage. The
	// usage path is what catches a program that calls extractor.X[T] but
	// dropped its `import ".../extractor"` — a missing-import regression.
	return importsModule || espressoUsageRe.MatchString(fence)
}

// startsWithPackageMain reports whether the first non-blank, non-comment line of
// the fence is the `package main` clause.
func startsWithPackageMain(fence string) bool {
	for _, line := range strings.Split(fence, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") {
			continue
		}
		return t == "package main"
	}
	return false
}

// importResolvable reports whether an import path resolves offline: it is either
// a real stdlib package or rooted at this module's path.
func importResolvable(path string, stdlib map[string]bool) bool {
	if path == modulePath || strings.HasPrefix(path, modulePath+"/") {
		return true
	}
	return stdlib[path]
}

// buildSnippet writes the fence into its own package dir under buildRoot and runs
// `go build` on it, failing the test (with the source doc + fence index) if it
// does not compile.
func buildSnippet(t *testing.T, buildRoot, doc string, idx int, fence string) {
	t.Helper()
	dir, err := os.MkdirTemp(buildRoot, "snip-")
	if err != nil {
		t.Fatalf("%s fence #%d: temp dir: %v", doc, idx, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fence), 0o600); err != nil {
		t.Fatalf("%s fence #%d: writing snippet: %v", doc, idx, err)
	}
	cmd := exec.Command("go", "build", "./"+filepath.ToSlash(dir))
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s fence #%d failed to compile: %v\n%s\n--- snippet ---\n%s",
			doc, idx, err, out, fence)
	}
}
