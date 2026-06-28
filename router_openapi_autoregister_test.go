package espresso

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestAutoRegisterRemoved is a source/godoc guard (D5, v2.3 task-03) that fails
// if the no-op AutoRegister stub or its misleading "registers all routes" godoc
// reappears. AutoRegister was an exported no-op whose godoc promised it
// registered every route on a Router into the spec; it did nothing, so it was
// deleted. This guard is stdlib-only (go/parser over the package source, no
// subprocess) so it stays cheap and dependency-free, and it is a compile guard
// at the same time: the package would not build if any code still referenced the
// removed symbol.
func TestAutoRegisterRemoved(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "router_openapi.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse router_openapi.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Only top-level functions (no receiver) carry the package-level
		// AutoRegister name; method receivers are scoped differently but we
		// reject any spelling to be safe.
		if fn.Name.Name == "AutoRegister" {
			t.Fatalf("AutoRegister was re-added to router_openapi.go; the no-op stub must stay deleted (D5)")
		}
	}

	// The misleading godoc phrasing must not reappear anywhere in the file's
	// comments either (it described behavior the symbol never had).
	for _, group := range file.Comments {
		text := group.Text()
		if strings.Contains(text, "AutoRegister registers all routes") {
			t.Fatalf("the misleading AutoRegister godoc reappeared; it must stay deleted (D5)")
		}
	}
}
