// Package extractor provides request extraction utilities for the espresso framework.
//
// Extractors parse and validate data from HTTP requests into typed structs.
// Built-in extractors include JSON, XML, Query, Form, Path, Header, and RawBody.
//
// Example:
//
//	import (
//	    "github.com/suryakencana007/espresso"
//	    "github.com/suryakencana007/espresso/extractor"
//	)
//
//	func handler(ctx context.Context, req extractor.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
//	    user := req.Data
//	    return espresso.JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
package extractor
