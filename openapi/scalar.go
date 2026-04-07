// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import "net/http"

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>API Reference</title>
    <style>
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <script id="api-reference" data-url="SPEC_URL_PLACEHOLDER"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// ScalarUIHandler returns an http.Handler that serves Scalar API Reference UI.
// Scalar provides a beautiful, modern API documentation interface.
// See: https://github.com/scalar/scalar
func ScalarUIHandler(specURL string) http.Handler {
	const specPlaceholder = "SPEC_URL_PLACEHOLDER"
	html := replaceString(scalarHTML, specPlaceholder, specURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})
}

// ScalarOpts contains options for Scalar UI.
type ScalarOpts struct {
	Title        string
	SpecURL      string
	BasePath     string
	Theme        string // default: "purple", options: "alternate", "moon", "purple", "solarized", "bluePlanet", "saturn", "kepler", "mars", "deepSpace"
	DarkMode     bool
	Favicon      string
	HideModels   bool
	HideDownload bool
}

// ScalarUI returns an HTTP handler that serves Scalar API Reference UI.
func ScalarUI(opts ScalarOpts) http.Handler {
	return ScalarUIHandler(opts.SpecURL)
}

// replaceString is a simple string replacement helper.
func replaceString(s, old, new string) string {
	for i := 0; i < len(s)-len(old)+1; i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
