// Package extractor provides request extraction utilities for the espresso framework.
//
// Extractors parse and validate data from HTTP requests into typed structs.
// Built-in extractors include JSON, XML, Query, Form, Path, Header, and RawBody.
//
// Handlers take extractors by POINTER — a value-typed argument fails to satisfy
// FromRequest and the framework panics at route registration.
//
// Example:
//
//	import (
//	    "github.com/suryakencana007/espresso/v2"
//	    "github.com/suryakencana007/espresso/v2/extractor"
//	)
//
//	func handler(ctx context.Context, req *extractor.Query[SearchReq]) (espresso.JSON[Results], error) {
//	    return espresso.JSON[Results]{Data: search(req.Data)}, nil
//	}
package extractor
