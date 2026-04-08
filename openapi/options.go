// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

// OperationOption customizes an OpenAPI operation.
type OperationOption func(*Operation)

// Summary sets the operation summary.
//
// Example:
//
//	router.Get("/users", handler, openapi.Summary("List all users"))
func Summary(summary string) OperationOption {
	return func(op *Operation) {
		op.Summary = summary
	}
}

// Description sets the operation description.
//
// Example:
//
//	router.Get("/users", handler, openapi.Description("Returns a paginated list of users"))
func Description(desc string) OperationOption {
	return func(op *Operation) {
		op.Description = desc
	}
}

// Tags sets the operation tags.
//
// Example:
//
//	router.Get("/users", handler, openapi.Tags("users"), openapi.Tags("admin"))
func Tags(tags ...string) OperationOption {
	return func(op *Operation) {
		op.Tags = append(op.Tags, tags...)
	}
}

// Security adds security requirements to the operation.
//
// Example:
//
//	router.Get("/users", handler, openapi.Security("bearerAuth"))
func Security(schemes ...string) OperationOption {
	return func(op *Operation) {
		security := make([]map[string][]string, len(schemes))
		for i, scheme := range schemes {
			security[i] = map[string][]string{scheme: {}}
		}
		op.Security = security
	}
}

// Status adds a response for a specific status code.
//
// Example:
//
//	router.Post("/users", handler, openapi.Status("201", openapi.Response{
//	    Description: "User created",
//	}))
func Status(code string, response Response) OperationOption {
	return func(op *Operation) {
		if op.Responses == nil {
			op.Responses = make(map[string]Response)
		}
		op.Responses[code] = response
	}
}

// Deprecated marks the operation as deprecated.
//
// Example:
//
//	router.Get("/old-endpoint", handler, openapi.Deprecated())
func Deprecated() OperationOption {
	return func(op *Operation) {
		// OpenAPI 3.0 doesn't have a deprecated field on operation,
		// but we can add it as an extension
		if op.Description != "" {
			op.Description += " (Deprecated)"
		} else {
			op.Description = "Deprecated"
		}
	}
}

// AddParam adds a parameter to the operation.
// Useful for manually documenting parameters that can't be auto-detected.
//
// Example:
//
//	router.Get("/search", handler,
//	    openapi.AddParam("q", "query", true, &openapi.Schema{Type: "string"}),
//	)
func AddParam(name, in string, required bool, schema *Schema) OperationOption {
	return func(op *Operation) {
		op.Parameters = append(op.Parameters, Parameter{
			Name:     name,
			In:       in,
			Required: required,
			Schema:   schema,
		})
	}
}

// AddResponse sets responses for the operation.
// This is a convenience function for setting multiple responses.
//
// Example:
//
//	router.Get("/users", handler, openapi.AddResponse("200", openapi.Response{
//	    Description: "List of users",
//	}))
func AddResponse(code string, response Response) OperationOption {
	return func(op *Operation) {
		if op.Responses == nil {
			op.Responses = make(map[string]Response)
		}
		op.Responses[code] = response
	}
}

// ApplyOptions applies multiple options to an operation.
func ApplyOptions(op *Operation, opts ...OperationOption) {
	for _, opt := range opts {
		opt(op)
	}
}
