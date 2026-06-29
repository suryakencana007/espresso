package openapi

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// rootImportPath is the import path of the root espresso package. The openapi
// package must never import it: espresso imports openapi, so the reverse edge
// would form an import cycle. introspect.go matches the root JSON[T]/State[T]
// extractors by package-path + base-name string precisely to avoid this edge.
const rootImportPath = "github.com/suryakencana007/espresso/v2"

// TestOpenAPIDoesNotImportRoot is an in-test import-direction guard (v2.3
// task-06): it parses every non-test .go file in the openapi package and fails
// if any imports the root espresso package, which would create an import cycle.
// This is stdlib-only (go/parser over the package source, no subprocess) so it
// stays cheap and dependency-free, and it complements the compile-time guarantee
// — the package would not build if the cycle existed, but this names the offender
// explicitly for the reviewer rather than failing as an opaque cycle error.
//
// Sub-package imports under the root (e.g. .../v2/extractor, .../v2/internal/...)
// are allowed and expected; only the bare root path is forbidden.
func TestOpenAPIDoesNotImportRoot(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse openapi package source: %v", err)
	}

	for pkgName, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s: unquote import %q: %v", fileName, imp.Path.Value, err)
				}
				if path == rootImportPath {
					t.Errorf("%s (package %s) imports the root package %q — this forms an import cycle (espresso imports openapi). Match root types by package-path + base-name instead (see introspect.go).",
						fileName, pkgName, rootImportPath)
				}
			}
		}
	}
}
