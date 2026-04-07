// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import "net/http"

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: window.location.pathname.replace(/\/$/, '') + "/openapi.json",
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "StandaloneLayout",
                deepLinking: true,
                displayOperationId: true,
                displayRequestDuration: true,
                docExpansion: "list",
                filter: true,
                showExtensions: true,
                showCommonExtensions: true,
            })
            window.ui = ui
        }
    </script>
</body>
</html>`

// SwaggerUIHandler returns an http.Handler that serves Swagger UI.
func SwaggerUIHandler(specURL string) http.Handler {
	_ = specURL // Dynamic URL is already in the template
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})
}

// SwaggerOpts contains options for Swagger UI.
type SwaggerOpts struct {
	Title       string
	SpecURL     string
	BasePath    string
	DeepLinking bool
}

// SwaggerUI returns an HTTP handler that serves Swagger UI.
func SwaggerUI(opts SwaggerOpts) http.Handler {
	return SwaggerUIHandler(opts.SpecURL)
}
