package espresso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suryakencana007/espresso/v2/extractor"
	httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"
)

// ============================================
// Handler-Signature Matrix (v2.2 task-04)
// ============================================
//
// Every supported handler shape — reflection, typed, and coffee aliases —
// registered on one Router and exercised through ServeHTTP, asserting each
// yields a 2xx with the expected body. This is the dispatch-side companion to
// the error matrix: it proves no supported shape regressed while the error
// pipeline was reworked. It also locks the task-01 outcome (two-extractor
// reflection fails fast at registration) and proves the request-time
// "this is a bug" panic is unreachable for every registered shape.

// sigReq is a body-less extractor (Extract is a no-op) so reflection rows that
// take *Req do not require a request body. Pointer receiver per the FromRequest
// contract.
type sigReq struct {
	Marker string
}

func (s *sigReq) Extract(*http.Request) error {
	s.Marker = "extracted"
	return nil
}

// sigPath is the first extractor for the two-extractor (Lungo) row: a path
// parameter. sigBody (JSON) is the second.
type sigPath struct {
	ID string `path:"id"`
}

type sigBody struct {
	Name string `json:"name"`
}

// echoRes echoes a marker string as JSON so each row can assert its body.
type echoRes struct {
	Where string `json:"where"`
}

func okJSON(where string) JSON[echoRes] { return JSON[echoRes]{Data: echoRes{Where: where}} }

// registerSignatureRoutes wires every supported shape and returns the route
// table (method, path, body, expected "where" marker) the tests drive.
type sigRoute struct {
	name   string
	method string
	path   string
	body   string
	// want is the expected echoRes.Where value (empty means "do not check body").
	want string
}

//nolint:funlen // a registration table for every handler shape is inherently long.
func registerSignatureRoutes(r *Router) []sigRoute {
	// --- reflection path ---
	// func() T
	r.Get("/refl/noreq", func() JSON[echoRes] { return okJSON("refl-noreq") })
	// func(*Req) T
	r.Get("/refl/req", func(req *sigReq) JSON[echoRes] { return okJSON("refl-req-" + req.Marker) })
	// func(ctx, *Req) (T, error)
	r.Post("/refl/ctxreqerr", func(ctx context.Context, req *JSON[sigBody]) (JSON[echoRes], error) {
		return okJSON("refl-ctxreqerr-" + req.Data.Name), nil
	})

	// --- typed path ---
	r.Post("/typed/ctxreqerr", HandlerCtxReqErr(
		func(ctx context.Context, req *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("typed-ctxreqerr-" + req.Data.Name), nil
		}))
	r.Post("/typed/ctxreq", HandlerCtxReq(
		func(ctx context.Context, req *JSON[sigBody]) JSON[echoRes] {
			return okJSON("typed-ctxreq-" + req.Data.Name)
		}))
	r.Post("/typed/reqerr", HandlerReqErr(
		func(req *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("typed-reqerr-" + req.Data.Name), nil
		}))
	r.Post("/typed/req", HandlerReq(
		func(req *JSON[sigBody]) JSON[echoRes] {
			return okJSON("typed-req-" + req.Data.Name)
		}))
	r.Get("/typed/ctxnoerr", HandlerCtxNoErr(
		func(ctx context.Context) JSON[echoRes] { return okJSON("typed-ctxnoerr") }))

	// --- coffee aliases ---
	r.Get("/alias/ristretto", Ristretto(
		func(ctx context.Context) JSON[echoRes] { return okJSON("ristretto") }))
	r.Post("/alias/solo", Solo(
		func(req *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("solo-" + req.Data.Name), nil
		}))
	r.Post("/alias/doppio", Doppio(
		func(ctx context.Context, req *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("doppio-" + req.Data.Name), nil
		}))
	// Lungo / HandlerCtxReq1Req2Err: BOTH extractors must populate.
	r.Put("/alias/lungo/{id}", Lungo(
		func(ctx context.Context, p *extractor.Path[sigPath], b *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("lungo-" + p.Data.ID + "-" + b.Data.Name), nil
		}))
	// HandlerCtxReq1Req2Err directly (same two-extractor path, typed constructor).
	r.Put("/typed/req1req2err/{id}", HandlerCtxReq1Req2Err(
		func(ctx context.Context, p *extractor.Path[sigPath], b *JSON[sigBody]) (JSON[echoRes], error) {
			return okJSON("req1req2err-" + p.Data.ID + "-" + b.Data.Name), nil
		}))

	return []sigRoute{
		{"refl_noreq", http.MethodGet, "/refl/noreq", "", "refl-noreq"},
		{"refl_req", http.MethodGet, "/refl/req", "", "refl-req-extracted"},
		{"refl_ctxreqerr", http.MethodPost, "/refl/ctxreqerr", `{"name":"alice"}`, "refl-ctxreqerr-alice"},
		{"typed_ctxreqerr", http.MethodPost, "/typed/ctxreqerr", `{"name":"bob"}`, "typed-ctxreqerr-bob"},
		{"typed_ctxreq", http.MethodPost, "/typed/ctxreq", `{"name":"carol"}`, "typed-ctxreq-carol"},
		{"typed_reqerr", http.MethodPost, "/typed/reqerr", `{"name":"dave"}`, "typed-reqerr-dave"},
		{"typed_req", http.MethodPost, "/typed/req", `{"name":"erin"}`, "typed-req-erin"},
		{"typed_ctxnoerr", http.MethodGet, "/typed/ctxnoerr", "", "typed-ctxnoerr"},
		{"alias_ristretto", http.MethodGet, "/alias/ristretto", "", "ristretto"},
		{"alias_solo", http.MethodPost, "/alias/solo", `{"name":"frank"}`, "solo-frank"},
		{"alias_doppio", http.MethodPost, "/alias/doppio", `{"name":"grace"}`, "doppio-grace"},
		{"alias_lungo_two_extractor", http.MethodPut, "/alias/lungo/42", `{"name":"heidi"}`, "lungo-42-heidi"},
		{"typed_req1req2err_two_extractor", http.MethodPut, "/typed/req1req2err/7", `{"name":"ivan"}`, "req1req2err-7-ivan"},
	}
}

// newSigRequest builds the request for a route row, setting Content-Type for
// bodied requests and the path value for the two-extractor rows (ServeMux sets
// it automatically on a matched pattern, but httptest direct dispatch goes
// through the mux too, so {id} is populated by the pattern match).
func newSigRequest(route sigRoute) *http.Request {
	var req *http.Request
	if route.body != "" {
		req = httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(route.method, route.path, nil)
	}
	return req
}

func TestHandlerSignatureMatrix(t *testing.T) {
	r := Portafilter()
	routes := registerSignatureRoutes(r)

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, newSigRequest(route))

			if rec.Code < 200 || rec.Code >= 300 {
				t.Fatalf("expected 2xx, got %d; body=%s", rec.Code, rec.Body.String())
			}

			if route.want == "" {
				return
			}
			var body JSON[echoRes]
			if err := json.Unmarshal(rec.Body.Bytes(), &body.Data); err != nil {
				t.Fatalf("response not JSON echoRes: %v; body=%s", err, rec.Body.String())
			}
			if body.Data.Where != route.want {
				t.Errorf("body.where: got %q, want %q", body.Data.Where, route.want)
			}
		})
	}
}

// TestTwoExtractorReflectionPanicsAtRegistration locks the task-01 fail-fast
// outcome (approach B): registering a reflection-path func(ctx, *Req1, *Req2)
// PANICS at registration time, and the panic message names the typed
// alternative (HandlerCtxReq1Req2Err) so users know how to fix it. The typed
// Lungo equivalent for the same shape works (asserted in the signature matrix).
func TestTwoExtractorReflectionPanicsAtRegistration(t *testing.T) {
	twoExtractorReflection := func(ctx context.Context, p *extractor.Path[sigPath], b *JSON[sigBody]) (JSON[echoRes], error) {
		return okJSON("unreachable"), nil
	}

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected registration to panic for a reflection-path two-extractor handler")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("expected string panic value, got %T: %v", rec, rec)
		}
		// The message must point users at the typed constructor.
		if !strings.Contains(msg, "HandlerCtxReq1Req2Err") {
			t.Errorf("panic message must name HandlerCtxReq1Req2Err, got %q", msg)
		}
		if !strings.Contains(msg, "FromRequest") {
			t.Errorf("panic message should explain the two-FromRequest-argument cause, got %q", msg)
		}
	}()

	// Registration path: Handler(any) runs the reflection analysis and must panic.
	_ = Handler(twoExtractorReflection)
}

// TestNoRequestTimeInvalidArgumentPanic exhaustively fires one request at every
// successfully registered shape and confirms NONE yields the request-time
// "this is a bug" 500 from createHandlerFromInfo (handler.go). Because that
// defensive panic would be recovered by RecoverMiddleware into a 500/PANIC
// envelope, the router here installs RecoverMiddleware and the test asserts no
// shape produces a PANIC envelope (or any 5xx).
func TestNoRequestTimeInvalidArgumentPanic(t *testing.T) {
	r := Portafilter()
	// RequestID + Recover so a hypothetical request-time bug-panic surfaces as a
	// catchable 500/PANIC envelope rather than crashing the goroutine. The
	// createHandlerFromInfo "this is a bug" panic (handler.go) is the specific
	// guard under test: it must be unreachable for every registered shape.
	r.Use(httpmiddleware.RequestIDMiddleware(), httpmiddleware.RecoverMiddleware())
	routes := registerSignatureRoutes(r)

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, newSigRequest(route))

			// Every shape here is expected to succeed; a 5xx implies the request
			// path hit the defensive bug-panic (recovered into 500/PANIC).
			if rec.Code >= 500 {
				t.Fatalf("shape %s produced a %d (possible request-time bug-panic); body=%s",
					route.name, rec.Code, rec.Body.String())
			}
			// Defensive: no response may surface the bug-panic string.
			if strings.Contains(rec.Body.String(), "this is a bug") {
				t.Fatalf("shape %s response mentions the bug-panic string; body=%s",
					route.name, rec.Body.String())
			}
		})
	}
}
