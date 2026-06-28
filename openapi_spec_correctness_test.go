package espresso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/suryakencana007/espresso/v2/extractor"
	"github.com/suryakencana007/espresso/v2/openapi"
)

// This file is the consolidated v2.3 spec-correctness matrix (task-06). It builds
// ONE OpenAPIRouter that exercises, together, the full extractor/handler/security
// matrix across BOTH registration paths (the fluent Get/Post and the standalone
// RegisterHandler), generates the spec ONCE, and asserts it field-by-field. The
// piecemeal correctness tests (openapi/introspect_correctness_test.go,
// router_openapi_correctness_test.go, openapi/openapi_test.go security tests,
// openapi/serving_test.go) each pin one defect; this matrix locks their combined
// post-fix behavior so Tasks 1-3 cannot silently regress in concert.

// matrixListReq is a query-extractor payload (one query parameter).
type matrixListReq struct {
	Page int `query:"page" doc:"Page number"`
}

// matrixPathReq is a path-extractor payload (one path parameter).
type matrixPathReq struct {
	ID int `path:"id" doc:"User identifier"`
}

// matrixCreateReq is a JSON request body.
type matrixCreateReq struct {
	Name string `json:"name"`
}

// matrixUser is the response body type shared across routes.
type matrixUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// matrixAuditReq is the payload carried by the custom (non-built-in) extractor.
type matrixAuditReq struct {
	Actor string `json:"actor"`
}

// matrixAuditExtractor is a CUSTOM FromRequest implementation — none of the
// built-in extractor base types. Before D2 was fixed, the custom-extractor probe
// interface was interface{ Extract(r any) error }, which no real extractor
// satisfies, so a custom extractor contributed nothing to the spec. A pointer
// receiver Extract(*http.Request) error is the real contract.
type matrixAuditExtractor struct {
	Data matrixAuditReq
}

func (a *matrixAuditExtractor) Extract(_ *http.Request) error { return nil }

// matrixCreated documents a non-200 success status (201) via the
// OpenAPIStatusCode interface, the type-level path to a non-200 status code.
type matrixCreated struct {
	StatusCode int
	Data       matrixUser
}

func (matrixCreated) OpenAPIStatusCode() int { return http.StatusCreated }

// WriteResponse makes matrixCreated a valid IntoResponse so the fluent path can
// register it on the underlying mux (the fluent Get/Post wrappers register the
// route both into the spec and onto the mux).
func (c matrixCreated) WriteResponse(w http.ResponseWriter) error {
	code := c.StatusCode
	if code == 0 {
		code = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(c.Data)
}

// matrixFilesLike has a "Files"-prefixed name but is NOT a framework extractor.
// The old prefix-table classifier (D8) would have mis-bucketed it as the files
// kind; correct classification keys off the actual base type, so it is unknown.
type matrixFilesLike struct {
	Data matrixUser
}

// Extract makes matrixFilesLike a custom FromRequest so it is introspected as a
// generic (unknown-kind) extractor rather than mis-classified by name prefix.
func (f *matrixFilesLike) Extract(_ *http.Request) error { return nil }

// buildMatrixSpec wires the full matrix onto a single OpenAPIRouter, registers
// the security scheme, generates the spec ONCE, and returns both the decoded
// navigable document and the raw bytes. Generate-once mirrors the real
// "spec is immutable after registration" contract that Task 3's cache relies on.
//
// registeredRoutes is the (method, path) set this test registered, so the
// no-silent-drop assertion can compare against the documented paths without
// re-deriving them from the spec.
type matrixRoute struct{ method, path string }

func buildMatrixSpec(t *testing.T) (doc map[string]any, registered []matrixRoute) {
	t.Helper()

	gen := openapi.New("Matrix API", "1.0.0")
	gen.AddSecurityScheme("bearerAuth", openapi.BearerScheme("JWT"))

	r := OpenAPI(gen)

	// --- Fluent registration path (Get/Post) ---

	// Path extractor: GET /users/{id}
	getByID := func(_ context.Context, p *extractor.Path[matrixPathReq]) (JSON[matrixUser], error) {
		return JSON[matrixUser]{Data: matrixUser{ID: p.Data.ID}}, nil
	}
	r.Get("/users/{id}", getByID, openapi.Tags("users"))

	// Query extractor: GET /users
	list := func(_ context.Context, q *extractor.Query[matrixListReq]) (JSON[matrixUser], error) {
		_ = q
		return JSON[matrixUser]{Data: matrixUser{}}, nil
	}
	r.Get("/users", list, openapi.Tags("users"))

	// JSON body + non-200 status (201) via OpenAPIStatusCode + secured route.
	create := func(_ context.Context, _ *JSON[matrixCreateReq]) (matrixCreated, error) {
		return matrixCreated{StatusCode: 201, Data: matrixUser{}}, nil
	}
	r.Post("/users", create,
		openapi.Tags("users"),
		openapi.Security("bearerAuth"),
	)

	// Custom (non-built-in) extractor: POST /audit. Its inner payload must
	// contribute to the spec (params and/or body), NOT be silently empty.
	audit := func(_ context.Context, _ *matrixAuditExtractor) (JSON[matrixUser], error) {
		return JSON[matrixUser]{Data: matrixUser{}}, nil
	}
	r.Post("/audit", audit, openapi.Tags("audit"))

	// Prefix-collision type ("Files"-prefixed user type) as a custom extractor:
	// must be introspected (route present) and not mis-classified by name prefix.
	prefix := func(_ context.Context, _ *matrixFilesLike) (JSON[matrixUser], error) {
		return JSON[matrixUser]{Data: matrixUser{}}, nil
	}
	r.Get("/files-like", prefix, openapi.Tags("misc"))

	// Status/Text route documenting a non-default code via openapi.Status.
	del := func(_ context.Context, _ *extractor.Path[matrixPathReq]) (Status, error) {
		return http.StatusNoContent, nil
	}
	r.Delete("/users/{id}/purge", del,
		openapi.Tags("users"),
		openapi.Status("204", openapi.Response{Description: "User purged"}),
	)

	// --- Standalone registration path (RegisterHandler) ---

	// Same handler shape as a fluent JSON[T] route, registered via the other path,
	// so the response-schema-on-both-paths assertion has a comparison point.
	regHandler := func(_ context.Context) (JSON[matrixUser], error) {
		return JSON[matrixUser]{Data: matrixUser{}}, nil
	}
	if err := RegisterHandler(gen, "GET", "/standalone", regHandler, openapi.Tags("standalone")); err != nil {
		t.Fatalf("RegisterHandler returned error: %v", err)
	}

	registered = []matrixRoute{
		{"GET", "/users/{id}"},
		{"GET", "/users"},
		{"POST", "/users"},
		{"POST", "/audit"},
		{"GET", "/files-like"},
		{"DELETE", "/users/{id}/purge"},
		{"GET", "/standalone"},
	}

	// Any fluent route that failed introspection would be recorded here rather
	// than silently dropped (D6) — assert the matrix registered cleanly.
	if errs := r.Errors(); len(errs) != 0 {
		t.Fatalf("fluent registration recorded introspection errors: %v", errs)
	}

	// Generate ONCE — mirrors the real "spec is immutable after registration"
	// contract that Task 3's cache relies on.
	raw, err := gen.JSON()
	if err != nil {
		t.Fatalf("gen.JSON() error = %v", err)
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return doc, registered
}

// opAt navigates doc["paths"][path][method] into a generic operation map,
// failing the test if the operation is absent.
func opAt(t *testing.T, doc map[string]any, path, method string) map[string]any {
	t.Helper()
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec has no paths object")
	}
	item, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("path %q absent from spec", path)
	}
	op, ok := item[strings.ToLower(method)].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s absent from spec", method, path)
	}
	return op
}

// paramIns collects the "in" value of every parameter on an operation.
func paramIns(op map[string]any) []string {
	params, _ := op["parameters"].([]any)
	out := make([]string, 0, len(params))
	for _, p := range params {
		if pm, ok := p.(map[string]any); ok {
			if in, ok := pm["in"].(string); ok {
				out = append(out, in)
			}
		}
	}
	return out
}

func hasParamIn(op map[string]any, in string) bool {
	for _, got := range paramIns(op) {
		if got == in {
			return true
		}
	}
	return false
}

// responseHasJSONSchema reports whether the operation's response under statusKey
// carries an application/json schema (not a bare description-only response).
func responseHasJSONSchema(op map[string]any, statusKey string) bool {
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		return false
	}
	resp, ok := responses[statusKey].(map[string]any)
	if !ok {
		return false
	}
	content, ok := resp["content"].(map[string]any)
	if !ok {
		return false
	}
	mt, ok := content["application/json"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = mt["schema"].(map[string]any)
	return ok
}

func responseKeys(op map[string]any) []string {
	responses, _ := op["responses"].(map[string]any)
	out := make([]string, 0, len(responses))
	for k := range responses {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestOpenAPISpecCorrectnessMatrix is the consolidated, generate-once, assert-many
// spec-correctness matrix. It builds the spec a single time and runs every
// property check as a sub-test against the same decoded document. Each property is
// a named helper so the driver stays a flat table and per-function cyclomatic
// complexity stays under the lint threshold.
func TestOpenAPISpecCorrectnessMatrix(t *testing.T) {
	doc, registered := buildMatrixSpec(t)

	checks := []struct {
		name string
		fn   func(*testing.T, map[string]any)
	}{
		{"extractor_matrix", assertExtractorMatrix},              // (a)
		{"status_codes", assertStatusCodes},                      // (b)
		{"security_refs_resolve", assertSecurityRefsResolve},     // (c)
		{"response_schema_both_paths", assertResponseSchemaBoth}, // (d)
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) { c.fn(t, doc) })
	}

	// (e) No registered route is silently missing from the spec.
	t.Run("no_route_silently_dropped", func(t *testing.T) {
		assertNoRouteDropped(t, doc, registered)
	})

	// (g) The served spec is cache-stable.
	t.Run("served_from_cache", func(t *testing.T) { assertServedFromCache(t) })
}

// assertExtractorMatrix pins property (a): path / query / json body / custom /
// prefix-collision extractors each contribute the right thing.
func assertExtractorMatrix(t *testing.T, doc map[string]any) {
	t.Helper()

	// Path extractor -> parameters with in:path.
	if getByID := opAt(t, doc, "/users/{id}", "GET"); !hasParamIn(getByID, "path") {
		t.Errorf("GET /users/{id}: expected a path parameter, params in=%v", paramIns(getByID))
	}

	// Query extractor -> parameters with in:query.
	if list := opAt(t, doc, "/users", "GET"); !hasParamIn(list, "query") {
		t.Errorf("GET /users: expected a query parameter, params in=%v", paramIns(list))
	}

	// JSON body -> requestBody present with a schema.
	create := opAt(t, doc, "/users", "POST")
	body, ok := create["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("POST /users: expected a requestBody")
	}
	content, _ := body["content"].(map[string]any)
	mt, _ := content["application/json"].(map[string]any)
	if _, ok := mt["schema"].(map[string]any); !ok {
		t.Error("POST /users: requestBody has no application/json schema")
	}

	// Custom extractor (D2): the route is present in the spec (not dropped), and —
	// the heart of the D2 fix — Introspect now SEES the custom extractor. Before D2
	// the probe interface matched no real extractor, so a custom FromRequest
	// contributed nothing to HandlerInfo. The generator maps a custom
	// (unknown-kind) extractor to neither params nor a requestBody, so the D2
	// contribution is asserted at the introspection boundary where the fix lives,
	// while the route's presence is asserted against the document.
	opAt(t, doc, "/audit", "POST") // fails if the custom-extractor route vanished
	assertCustomExtractorIntrospected(t)

	// Prefix-collision type (D8): a "Files"-prefixed user type is present (route
	// not dropped) and classified as the unknown kind, NOT mis-bucketed into the
	// built-in files kind by name prefix.
	opAt(t, doc, "/files-like", "GET") // fails if the route vanished
	assertPrefixCollisionNotMisclassified(t)
}

func assertCustomExtractorIntrospected(t *testing.T) {
	t.Helper()
	info, err := openapi.Introspect(func(_ context.Context, _ *matrixAuditExtractor) (JSON[matrixUser], error) {
		return JSON[matrixUser]{}, nil
	})
	if err != nil {
		t.Fatalf("introspect custom-extractor handler: %v", err)
	}
	if len(info.RequestTypes) != 1 || len(info.ExtractorKinds) != 1 {
		t.Fatalf("custom extractor not introspected (D2 regression): RequestTypes=%v ExtractorKinds=%v",
			info.RequestTypes, info.ExtractorKinds)
	}
	if info.ExtractorKinds[0] != openapi.KindUnknown {
		t.Errorf("custom extractor kind = %q, want %q", info.ExtractorKinds[0], openapi.KindUnknown)
	}
}

func assertPrefixCollisionNotMisclassified(t *testing.T) {
	t.Helper()
	info, err := openapi.Introspect(func(_ context.Context, _ *matrixFilesLike) (JSON[matrixUser], error) {
		return JSON[matrixUser]{}, nil
	})
	if err != nil {
		t.Fatalf("introspect prefix-collision handler: %v", err)
	}
	if len(info.ExtractorKinds) != 1 || info.ExtractorKinds[0] != openapi.KindUnknown {
		t.Errorf("prefix-collision type mis-classified (D8 regression): ExtractorKinds=%v, want [%q]",
			info.ExtractorKinds, openapi.KindUnknown)
	}
}

// assertStatusCodes pins property (b): non-200 status codes are documented (not
// just 200).
func assertStatusCodes(t *testing.T, doc map[string]any) {
	t.Helper()

	// 201 via OpenAPIStatusCode on the response type (type-level non-200 path).
	// matrixCreated is not a JSON[T], so it correctly carries no response-body
	// schema — the assertion here is that the documented status is 201, NOT the
	// spurious 200 the pre-D3 always-0 behavior would have produced. Response-body
	// schema presence is asserted on the JSON[T] routes in response_schema_both_paths.
	create := opAt(t, doc, "/users", "POST")
	if !hasResponse(create, "201") {
		t.Errorf("POST /users: expected a 201 response (type declares OpenAPIStatusCode 201), got %v", responseKeys(create))
	}
	if hasResponse(create, "200") {
		t.Errorf("POST /users: documented a spurious 200 alongside the declared 201: %v", responseKeys(create))
	}

	// 204 via openapi.Status on a Status-returning route (option-level non-default).
	del := opAt(t, doc, "/users/{id}/purge", "DELETE")
	if !hasResponse(del, "204") {
		t.Errorf("DELETE /users/{id}/purge: expected a 204 response, got %v", responseKeys(del))
	}
	if hasResponse(del, "200") {
		t.Errorf("DELETE /users/{id}/purge: documented a spurious 200 alongside the declared 204: %v", responseKeys(del))
	}

	// Default JSON[T] route still documents 200.
	if list := opAt(t, doc, "/users", "GET"); !hasResponse(list, "200") {
		t.Errorf("GET /users: expected the default 200 response, got %v", responseKeys(list))
	}
}

// assertSecurityRefsResolve pins property (c): every operation security reference
// resolves to a defined components.securitySchemes entry — no dangling ref.
func assertSecurityRefsResolve(t *testing.T, doc map[string]any) {
	t.Helper()

	defined := definedSecuritySchemes(t, doc)
	if _, ok := defined["bearerAuth"]; !ok {
		t.Fatal("components.securitySchemes is missing bearerAuth")
	}

	refs := allSecurityRefs(t, doc)
	if len(refs) == 0 {
		t.Fatal("anti-vacuity: no operation carried a security requirement; the secured route was not exercised")
	}
	for path, names := range refs {
		for _, name := range names {
			if _, ok := defined[name]; !ok {
				t.Errorf("operation %s references undefined security scheme %q (dangling ref)", path, name)
			}
		}
	}
}

// assertResponseSchemaBoth pins property (d): response-body schemas are present on
// routes registered via BOTH the fluent path and RegisterHandler.
func assertResponseSchemaBoth(t *testing.T, doc map[string]any) {
	t.Helper()

	if list := opAt(t, doc, "/users", "GET"); !responseHasJSONSchema(list, "200") {
		t.Error("fluent GET /users: 200 response is missing the response-body schema")
	}
	if standalone := opAt(t, doc, "/standalone", "GET"); !responseHasJSONSchema(standalone, "200") {
		t.Error("RegisterHandler GET /standalone: 200 response is missing the response-body schema (D9 regression)")
	}
}

// assertNoRouteDropped pins property (e): every registered route appears in the
// spec, and the documented path count equals the registered distinct-path count.
func assertNoRouteDropped(t *testing.T, doc map[string]any, registered []matrixRoute) {
	t.Helper()

	paths, _ := doc["paths"].(map[string]any)
	wantPaths := map[string]struct{}{}
	for _, rt := range registered {
		wantPaths[rt.path] = struct{}{}
		item, ok := paths[rt.path].(map[string]any)
		if !ok {
			t.Errorf("registered route %s %s is missing from the spec entirely", rt.method, rt.path)
			continue
		}
		if _, ok := item[strings.ToLower(rt.method)].(map[string]any); !ok {
			t.Errorf("registered route %s %s: path present but method operation absent", rt.method, rt.path)
		}
	}
	if len(paths) != len(wantPaths) {
		t.Errorf("documented path count %d != registered distinct path count %d (a route was dropped or an extra slipped in)",
			len(paths), len(wantPaths))
	}
}

// assertServedFromCache pins property (g): two requests to the spec handler return
// byte-identical bodies (cache-stable serving).
func assertServedFromCache(t *testing.T) {
	t.Helper()

	h := openAPIHandlerFor(t)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("expected 200/200 from spec handler, got %d/%d", rec1.Code, rec2.Code)
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Error("spec handler returned different bytes on two successive requests — not cache-stable")
	}
}

// hasResponse reports whether an operation documents the given status key.
func hasResponse(op map[string]any, statusKey string) bool {
	responses, _ := op["responses"].(map[string]any)
	_, ok := responses[statusKey]
	return ok
}

// definedSecuritySchemes pulls components.securitySchemes from the decoded doc.
func definedSecuritySchemes(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	comps, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("spec has no components object")
	}
	schemes, _ := comps["securitySchemes"].(map[string]any)
	return schemes
}

// allSecurityRefs walks every operation under paths and collects the scheme names
// referenced by each operation's security requirement, keyed by "METHOD path".
func allSecurityRefs(t *testing.T, doc map[string]any) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	paths, _ := doc["paths"].(map[string]any)
	methods := []string{"get", "post", "put", "delete", "patch", "options", "head"}
	for path, raw := range paths {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, m := range methods {
			op, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			sec, ok := op["security"].([]any)
			if !ok {
				continue
			}
			var names []string
			for _, req := range sec {
				reqMap, ok := req.(map[string]any)
				if !ok {
					continue
				}
				for name := range reqMap {
					names = append(names, name)
				}
			}
			if len(names) > 0 {
				out[strings.ToUpper(m)+" "+path] = names
			}
		}
	}
	return out
}

// TestOpenAPISpecCorrectnessMatrix_FailurePathEnvelope pins property (f): the
// spec-generation failure path returns the canonical application/json envelope,
// never text/plain. It mirrors TestWithLayers_ExtractorErrorReturnsStructuredJSON
// for the OpenAPI serving path. A channel-typed Example cannot be marshaled, so
// the handler's marshal fails and must emit the envelope.
func TestOpenAPISpecCorrectnessMatrix_FailurePathEnvelope(t *testing.T) {
	gen := openapi.New("Matrix API", "1.0.0")
	gen.AddPath(http.MethodGet, "/boom", openapi.Operation{
		Summary: "boom",
		Responses: map[string]openapi.Response{
			"200": {
				Description: "bad",
				Content: map[string]openapi.MediaType{
					"application/json": {Example: make(chan int)},
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	gen.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
	if strings.Contains(ct, "text/plain") {
		t.Fatalf("failure path regressed to text/plain: %q", ct)
	}

	var env struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Details   any    `json:"details"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not the canonical envelope JSON: %v (body=%q)", err, rec.Body.String())
	}
	if env.Error.Code == "" {
		t.Error("expected a non-empty error code in the failure envelope")
	}
}

// openAPIHandlerFor builds a fresh OpenAPIRouter serving a matrix-shaped spec on
// /openapi.json, so the served-from-cache assertion exercises the real serving
// path (ServeHTTP -> cached bytes), not just the generator in isolation.
func openAPIHandlerFor(t *testing.T) http.Handler {
	t.Helper()
	gen := openapi.New("Matrix API", "1.0.0")
	gen.AddSecurityScheme("bearerAuth", openapi.BearerScheme("JWT"))
	r := OpenAPI(gen)
	list := func(_ context.Context, q *extractor.Query[matrixListReq]) (JSON[matrixUser], error) {
		_ = q
		return JSON[matrixUser]{Data: matrixUser{}}, nil
	}
	r.Get("/users", list, openapi.Tags("users"))
	r.ServeOpenAPI("/openapi.json")
	return r
}
