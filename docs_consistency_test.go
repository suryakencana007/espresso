package espresso

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
