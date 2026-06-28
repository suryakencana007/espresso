package espresso

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/suryakencana007/espresso/v2/openapi"
)

type correctnessUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type correctnessCreateUser struct {
	Name string `json:"name"`
}

// TestOpenAPI_RealStatusCodeDocumented pins D3 end-to-end: a POST whose success
// status is declared 201 documents under the "201" response key (with the
// response-body schema), while a plain JSON[T] handler documents under "200".
func TestOpenAPI_RealStatusCodeDocumented(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")

	create := func(_ context.Context, _ *JSON[correctnessCreateUser]) (JSON[correctnessUser], error) {
		return JSON[correctnessUser]{StatusCode: 201, Data: correctnessUser{}}, nil
	}
	list := func(_ context.Context) JSON[correctnessUser] {
		return JSON[correctnessUser]{Data: correctnessUser{}}
	}

	OpenAPI(gen).
		Post("/users", create, openapi.Status("201", openapi.Response{Description: "User created"})).
		Get("/users", list)

	post := gen.Spec().Paths["/users"].Post
	if post == nil {
		t.Fatal("expected POST /users registered")
	}
	if _, ok := post.Responses["201"]; !ok {
		t.Errorf("expected 201 response key, got %v", keys(post.Responses))
	}
	if _, ok := post.Responses["200"]; ok {
		t.Errorf("did not expect a bare 200 response when 201 is declared: %v", keys(post.Responses))
	}
	if post.Responses["201"].Content["application/json"].Schema == nil {
		t.Error("expected 201 response to carry the response-body schema")
	}

	get := gen.Spec().Paths["/users"].Get
	if get == nil {
		t.Fatal("expected GET /users registered")
	}
	if _, ok := get.Responses["200"]; !ok {
		t.Errorf("expected default JSON handler to document 200, got %v", keys(get.Responses))
	}
}

// TestOpenAPI_RegisterHandlerAttachesResponseSchema pins D9: RegisterHandler now
// attaches the response-body schema, matching what the fluent register path
// produces for the same handler — no more bare 200:{description:"Success"}.
func TestOpenAPI_RegisterHandlerAttachesResponseSchema(t *testing.T) {
	handler := func(_ context.Context) (JSON[correctnessUser], error) {
		return JSON[correctnessUser]{Data: correctnessUser{}}, nil
	}

	// Fluent path.
	fluentGen := openapi.New("Test API", "1.0.0")
	OpenAPI(fluentGen).Get("/users", handler)
	fluent := fluentGen.Spec().Paths["/users"].Get

	// RegisterHandler path.
	regGen := openapi.New("Test API", "1.0.0")
	if err := RegisterHandler(regGen, "GET", "/users", handler); err != nil {
		t.Fatalf("RegisterHandler returned error: %v", err)
	}
	reg := regGen.Spec().Paths["/users"].Get

	if reg == nil || fluent == nil {
		t.Fatal("expected both register paths to produce an operation")
	}

	regResp := reg.Responses["200"]
	if regResp.Content == nil || regResp.Content["application/json"].Schema == nil {
		t.Fatal("RegisterHandler did not attach the response-body schema")
	}

	fluentResp := fluent.Responses["200"]
	if got, want := mustJSON(t, regResp), mustJSON(t, fluentResp); got != want {
		t.Errorf("RegisterHandler response = %s, fluent path = %s", got, want)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestOpenAPI_IntrospectErrorNotSilentlyDropped pins D6: a handler that fails
// introspection is recorded on the router (not silently dropped) via the fluent
// path, while RegisterHandler still returns the error.
func TestOpenAPI_IntrospectErrorNotSilentlyDropped(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")

	// A handler with too many return values fails introspection. registerPath
	// is exercised directly because the fluent Get/Post wrappers also register
	// the route on the underlying mux, which panics on an invalid signature —
	// D6 is specifically about registerPath not swallowing the introspect error.
	bad := func() (int, string, bool) { return 0, "", false }

	r := OpenAPI(gen)
	r.registerPath("GET", "/bad", bad)

	if len(r.Errors()) == 0 {
		t.Error("expected introspection failure recorded on router, got none")
	}
	if _, ok := gen.Spec().Paths["/bad"]; ok {
		t.Error("expected uninstrospectable route omitted from spec")
	}

	if err := RegisterHandler(gen, "GET", "/bad", bad); err == nil {
		t.Error("expected RegisterHandler to return the introspection error")
	}
}

// TestOpenAPI_SpecMatrix_Snapshot is the cross-cutting regression lock: a fixed
// route set exercising path + JSON body + 201 response is asserted field by
// field.
func TestOpenAPI_SpecMatrix_Snapshot(t *testing.T) {
	gen := openapi.New("Matrix API", "1.0.0")

	create := func(_ context.Context, _ *JSON[correctnessCreateUser]) (JSON[correctnessUser], error) {
		return JSON[correctnessUser]{StatusCode: 201}, nil
	}

	OpenAPI(gen).
		Post("/users", create,
			openapi.Tags("users"),
			openapi.Summary("Create user"),
			openapi.Status("201", openapi.Response{Description: "User created"}),
		)

	post := gen.Spec().Paths["/users"].Post
	if post == nil {
		t.Fatal("expected POST /users registered")
	}
	if post.Summary != "Create user" {
		t.Errorf("summary = %q, want %q", post.Summary, "Create user")
	}
	if len(post.Tags) != 1 || post.Tags[0] != "users" {
		t.Errorf("tags = %v, want [users]", post.Tags)
	}
	if post.RequestBody == nil || post.RequestBody.Content["application/json"].Schema == nil {
		t.Error("expected JSON request body schema")
	}
	resp, ok := post.Responses["201"]
	if !ok {
		t.Fatalf("expected 201 response, got %v", keys(post.Responses))
	}
	if resp.Content["application/json"].Schema == nil {
		t.Error("expected 201 response-body schema")
	}
	if _, ok := post.Responses["200"]; ok {
		t.Errorf("unexpected bare 200 response: %v", keys(post.Responses))
	}
}

func keys(m map[string]openapi.Response) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
