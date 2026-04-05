package main

import (
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing: %v\n", err)
		return
	}

	for pkgName, pkg := range pkgs {
		if strings.HasPrefix(pkgName, "middleware") || strings.HasPrefix(pkgName, "extractor") || strings.HasPrefix(pkgName, "pool") {
			continue // Skip subpackages, they have their own docs
		}

		d := doc.New(pkg, "./", doc.AllDecls|doc.AllTypes|doc.AllMethods)

		// Generate API doc
		output := generateAPIPage(d, pkgName)

		// Write to file
		outputPath := filepath.Join("docs", "api", pkgName+".md")
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			fmt.Printf("Error creating dir: %v\n", err)
			continue
		}

		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			continue
		}

		fmt.Printf("Generated: %s\n", outputPath)
	}

	// Generate index
	generateAPIIndex()
}

func generateAPIPage(d *doc.Package, pkgName string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`---
title: %s API Reference
description: %s package API documentation
---

# %s API Reference

`, d.Name, d.Name, d.Name))

	if d.Doc != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", cleanDoc(d.Doc)))
	}

	// Types
	if len(d.Types) > 0 {
		sb.WriteString("## Types\n\n")
		for _, t := range d.Types {
			sb.WriteString(fmt.Sprintf("### %s\n\n", t.Name))
			if t.Doc != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", cleanDoc(t.Doc)))
			}
		}
	}

	// Functions
	if len(d.Funcs) > 0 {
		sb.WriteString("## Functions\n\n")
		for _, f := range d.Funcs {
			if strings.HasPrefix(f.Name, "test") || strings.ToLower(f.Name[0:1]) == f.Name[0:1] {
				continue // Skip test and unexported functions
			}
			sb.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
			if f.Doc != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", cleanDoc(f.Doc)))
			}
		}
	}

	return sb.String()
}

func cleanDoc(doc string) string {
	// Clean up doc string
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

func generateAPIIndex() {
	var sb strings.Builder

	sb.WriteString(`---
title: API Reference
description: Espresso API Reference
---

# API Reference

Complete API reference for all Espresso packages.

## Core Packages

| Package | Description |
|---------|-------------|
| [espresso](/api/espresso) | Core - handlers, router, server |
| [extractor](/api/extractor) | Request extractors |
| [middleware/http](/api/middleware-http) | HTTP middleware |
| [middleware/service](/api/middleware-service) | Service layers |
| [pool](/api/pool) | Object pooling |

## Import Paths

```go
import (
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
    "github.com/suryakencana007/espresso/pool"
)
```
`)

	outputPath := filepath.Join("docs", "api", "index.md")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Printf("Error creating dir: %v\n", err)
		return
	}

	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("Generated: %s\n", outputPath)
}