package espresso

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ============================================
// Doc / Code Consistency (v2.2 task-04)
// ============================================
//
// Two guards that fail if a removed symbol or a false signature claim creeps
// back into the published docs or the exported godoc:
//
//  1. A forbidden-string scan over docs/*.md. Removed symbols (the v2.0/v2.1 SSE
//     API) must not reappear in *example code*. The migration guides and the
//     api/espresso.md reference legitimately NAME these symbols while documenting
//     their removal, so the scan (a) skips those files and (b) only inspects
//     ```go fenced code blocks, never prose. With both rules the scan is GREEN on
//     current main and only trips if someone pastes dead API into a doc example.
//
//  2. A godoc guard on handler.go's Handler comment. The comment legitimately
//     MENTIONS func(context.Context, *Req1, *Req2) while stating it is NOT
//     supported — so asserting mere absence of that substring would be wrong.
//     Instead this guard asserts the comment carries the CORRECTED language: it
//     says the two-extractor shape is not supported / rejected at registration
//     AND names the typed alternative HandlerCtxReq1Req2Err. This locks the PR #39
//     godoc fix against reintroduction of the old false "reflection supports it"
//     claim.

// docsForbiddenSymbols are strings that must not appear inside Go example code
// in the docs. They are symbols already retired from the public API; a doc
// example using them would mislead users. Seeded with the v2.0/v2.1 SSE removal.
var docsForbiddenSymbols = []string{
	"NewSSEWriter",
	"espresso.SSE{",
	"SSEWriter",
	"SSEEvent",
}

// docsScanSkip lists doc files that legitimately reference the retired symbols
// because their job is to document the removal/migration. Keyed by the path
// relative to docs/ using forward slashes.
var docsScanSkip = map[string]bool{
	"migration-v1-to-v2.md":   true,
	"migration-v2-to-v2.1.md": true,
	"api/espresso.md":         true,
}

func TestDocsConsistency(t *testing.T) {
	t.Run("forbidden_symbols_in_docs_examples", testDocsForbiddenSymbols)
	t.Run("handler_godoc_two_extractor_claim", testHandlerGodocClaim)
	t.Run("extractor_generics_qualified_with_extractor_pkg", testDocsExtractorQualifier)
	t.Run("corrected_claims_do_not_regress", testDocsCorrectedClaims)
}

// extractorGenericRe matches a root-package qualification of an extractor
// generic (espresso.Path[, espresso.Query[, …]). Those generics are exported by
// the extractor package, not the root package, so a doc using espresso.X[ would
// not compile. The negation here is intentional: the root JSON response generic
// (espresso.JSON[) is correct and must NOT match.
var extractorGenericRe = regexp.MustCompile(`espresso\.(Path|Query|Form|Header|XML)\[`)

// testDocsExtractorQualifier asserts no docs file qualifies an extractor generic
// with the root espresso package — the guard for the v2.3 task-04 sweep. It scans
// raw file content (not just fences) so a stray reference anywhere trips it.
func testDocsExtractorQualifier(t *testing.T) {
	docsRoot := "docs"
	if _, err := os.Stat(docsRoot); err != nil {
		t.Skipf("docs/ not present (%v); skipping extractor-qualifier scan", err)
	}

	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
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
		for i, line := range strings.Split(string(content), "\n") {
			if m := extractorGenericRe.FindString(line); m != "" {
				t.Errorf("extractor generic qualified with root package at %s:%d: %q "+
					"(use extractor.%s)", rel, i+1, strings.TrimSpace(line),
					strings.TrimSuffix(strings.TrimPrefix(m, "espresso."), "["))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs/: %v", err)
	}
}

// testDocsForbiddenSymbols walks docs/, and for every non-skipped markdown file
// scans only the contents of ```go fenced code blocks for the forbidden
// symbols, failing with the file + line on any hit.
func testDocsForbiddenSymbols(t *testing.T) {
	docsRoot := "docs"
	if _, err := os.Stat(docsRoot); err != nil {
		t.Skipf("docs/ not present (%v); skipping doc scan", err)
	}

	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip generated / build output directories.
			if name := d.Name(); name == ".vitepress" || name == "node_modules" || name == "dist" || name == "public" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		rel, relErr := filepath.Rel(docsRoot, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		if docsScanSkip[rel] {
			return nil
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // path comes from a trusted docs/ walk
		if readErr != nil {
			return readErr
		}
		scanGoFencesForForbidden(t, rel, string(content))
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs/: %v", err)
	}
}

// scanGoFencesForForbidden inspects only ```go fenced code blocks for the
// forbidden symbols. Prose (including inline-code mentions like the legitimate
// "older low-level SSEWriter API" note in streaming.md) is intentionally ignored.
func scanGoFencesForForbidden(t *testing.T, file, content string) {
	t.Helper()

	lines := strings.Split(content, "\n")
	inGoFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inGoFence {
				// Opening fence: enter Go mode only for ```go / ```golang.
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				inGoFence = lang == "go" || lang == "golang"
			} else {
				// Closing fence.
				inGoFence = false
			}
			continue
		}
		if !inGoFence {
			continue
		}
		for _, sym := range docsForbiddenSymbols {
			if strings.Contains(line, sym) {
				t.Errorf("forbidden symbol %q found in Go example at %s:%d: %s",
					sym, file, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// correctedClaim is a single audit finding whose fix Task 10 landed in v2.4.
// The pattern is a substring that must NOT reappear anywhere in the target
// files — reintroducing it means an editor pasted the pre-fix language back
// in. Sites lists the paths to scan (raw content, not just fences: some of
// these are prose claims); skips lists paths where the pattern legitimately
// appears (e.g. this test file names the forbidden phrases; the CHANGELOG
// documents the correction; the roadmap task files quote the old wording).
type correctedClaim struct {
	pattern string
	sites   []string
	skips   map[string]bool
	because string
}

// docsCorrectedClaims are the audit-flagged phrases whose fix landed in v2.4.
// Every entry maps to a specific PR that corrected it; a regression here
// means the correction was undone by a later edit.
var docsCorrectedClaims = []correctedClaim{
	{
		pattern: "Zero-allocation handlers",
		sites:   []string{"README.md", "docs/index.md", "docs/guide/index.md"},
		skips:   map[string]bool{"CHANGELOG.md": true},
		because: "v2.4 task-10 reworded to the defensible pooled-request claim (measured ~8 allocs/op for Doppio JSON, ~2 for Ristretto).",
	},
	{
		pattern: "zero-allocation object pooling",
		sites:   []string{"core.go"},
		because: "v2.4 task-10 reworded package doc to reflect that only the request-struct pool is genuinely zero-alloc.",
	},
	{
		pattern: "Zero per request",
		sites:   []string{"README.md"},
		because: "v2.4 task-10 replaced the Handler Performance table with measured framework-side numbers.",
	},
	{
		pattern: "Middleware runs in reverse order",
		sites:   []string{"docs/guide/middleware/index.md", "docs/examples/middleware-stack.md"},
		because: "v2.4 task-10 (PR #63) corrected middleware ordering docs: first-registered = outermost = executes first.",
	},
	{
		pattern: "last added = first executed",
		sites:   []string{"docs/guide/middleware/index.md"},
		because: "v2.4 task-10 (PR #63) corrected the backwards middleware-order claim.",
	},
	{
		pattern: "func WithServer(",
		sites:   []string{"docs/api/espresso.md"},
		because: "v2.4 task-10 (PR #63) deleted the phantom WithServer ServerOption that never existed.",
	},
	{
		pattern: "func GetState[T any](ctx context.Context) (T, error)",
		sites:   []string{"docs/api/state.md", "docs/api/index.md"},
		because: "v2.4 task-10 (PR #63) corrected GetState signature to (T, bool) — actual return type since introduction.",
	},
	{
		pattern: "func Solo[T any](f func(context.Context) T)",
		sites:   []string{"docs/api/espresso.md", "docs/api/index.md"},
		because: "v2.4 task-10 (PR #63) corrected Solo signature; ctx-only shape is Ristretto, Solo takes an extractor + err.",
	},
	{
		pattern: "Uses pooled byte slices",
		sites:   []string{"extractor/extractor.go"},
		because: "v2.4 task-10 removed the false pooling claim; RawBodyExtractor has no shared byte-slice pool.",
	},
	{
		pattern: "Use pooled buffer for encoding",
		sites:   []string{"response.go"},
		because: "v2.4 task-10 removed the false pooling comment; JSON.WriteResponse streams direct to ResponseWriter.",
	},
	{
		pattern: "verified under -race)",
		sites:   []string{"openapi/openapi.go"},
		skips:   map[string]bool{"openapi/openapi_test.go": true},
		because: "v2.4 task-05 (PR #71) tightened the Generator godoc — the old claim without a test reference was audit finding openapi-validator#1.",
	},
}

// testDocsCorrectedClaims scans each site file for its pattern and fails on
// a hit. Locks Task 10's corrections against silent regression: any future PR
// that reintroduces one of these phrases into a scanned file breaks CI with
// a message pointing at the audit finding and the corrective PR.
func testDocsCorrectedClaims(t *testing.T) {
	for _, c := range docsCorrectedClaims {
		for _, site := range c.sites {
			if c.skips[site] {
				continue
			}
			data, err := os.ReadFile(site) //nolint:gosec // sites are hard-coded relative paths
			if err != nil {
				if os.IsNotExist(err) {
					t.Logf("site %q not present; skipping (%v)", site, err)
					continue
				}
				t.Errorf("read %s: %v", site, err)
				continue
			}
			content := string(data)
			if !strings.Contains(content, c.pattern) {
				continue
			}
			// Report every offending line.
			for i, line := range strings.Split(content, "\n") {
				if strings.Contains(line, c.pattern) {
					t.Errorf("%s:%d reintroduced corrected claim %q — %s\n  offending line: %s",
						site, i+1, c.pattern, c.because, strings.TrimSpace(line))
				}
			}
		}
	}
}

// testHandlerGodocClaim parses handler.go and asserts the Handler function's
// doc comment carries the corrected two-extractor language: it states the shape
// is NOT supported / rejected at registration AND names HandlerCtxReq1Req2Err.
func testHandlerGodocClaim(t *testing.T) {
	const src = "handler.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", src, err)
	}

	pkg, err := doc.NewFromFiles(fset, []*ast.File{file}, "github.com/suryakencana007/espresso/v2")
	if err != nil {
		t.Fatalf("building godoc for %s: %v", src, err)
	}

	var handlerDoc string
	for _, fn := range pkg.Funcs {
		if fn.Name == "Handler" {
			handlerDoc = fn.Doc
			break
		}
	}
	if handlerDoc == "" {
		t.Fatalf("could not find godoc for the Handler function in %s", src)
	}

	// (1) Must name the typed alternative so users can migrate.
	if !strings.Contains(handlerDoc, "HandlerCtxReq1Req2Err") {
		t.Errorf("Handler godoc must name the typed alternative HandlerCtxReq1Req2Err; got:\n%s", handlerDoc)
	}

	// (2) Must state the two-extractor shape is NOT supported. Accept any of the
	// phrasings the corrected comment may use ("NOT supported", "rejected at
	// registration"); the comment legitimately mentions the signature, so we
	// assert on the negative-claim language, not the signature's absence.
	low := strings.ToLower(handlerDoc)
	hasNotSupported := strings.Contains(low, "not supported") ||
		strings.Contains(low, "are not supported") ||
		strings.Contains(low, "rejected at registration")
	if !hasNotSupported {
		t.Errorf("Handler godoc must state the two-extractor reflection shape is NOT supported "+
			"(or rejected at registration); got:\n%s", handlerDoc)
	}

	// (3) Guard against the old false positive claim reappearing verbatim.
	for _, falseClaim := range []string{
		"reflection path supports func(context.Context, *Req1, *Req2)",
		"reflection path supports func(ctx, *Req1, *Req2)",
		"two-extractor handlers are supported by this reflection path",
	} {
		if strings.Contains(handlerDoc, falseClaim) {
			t.Errorf("Handler godoc reintroduced the removed false claim %q", falseClaim)
		}
	}
}
