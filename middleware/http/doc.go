// Package httpmiddleware provides HTTP-level middleware for the espresso framework.
//
// HTTP middleware operates on raw HTTP requests before request extraction.
// This includes CORS handling, compression, rate limiting, authentication, etc.
//
// Example:
//
//	import (
//	    "github.com/suryakencana007/espresso/v2"
//	    "github.com/suryakencana007/espresso/v2/middleware/http"
//	)
//
//	app := espresso.Portafilter()
//	app.Use(httpmiddleware.CORS(httpmiddleware.DefaultCORSConfig))
//	app.Use(httpmiddleware.RequestID())
//	app.Use(httpmiddleware.Recover())
package httpmiddleware
