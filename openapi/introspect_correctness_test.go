package openapi

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/suryakencana007/espresso/v2/extractor"
)

// auditReq is the payload carried by a custom (non-built-in) extractor.
type auditReq struct {
	Actor string `json:"actor"`
}

// auditExtractor is a custom FromRequest implementation that is not one of the
// built-in extractors. Before D2 was fixed, the probe interface was
// interface{ Extract(r any) error }, which no real extractor satisfies, so this
// type was never introspected.
type auditExtractor struct {
	Data auditReq
}

func (a *auditExtractor) Extract(_ *http.Request) error { return nil }

// TestOpenAPI_CustomExtractorIntrospected pins D2: a custom extractor whose
// Extract(*http.Request) error is satisfied is now introspected and contributes
// to the handler info, where before the dead probe interface dropped it.
func TestOpenAPI_CustomExtractorIntrospected(t *testing.T) {
	handler := func(_ context.Context, _ *auditExtractor) (string, error) { return "ok", nil }

	info, err := Introspect(handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.RequestTypes) != 1 {
		t.Fatalf("expected custom extractor introspected (1 request type), got %d", len(info.RequestTypes))
	}
	if len(info.ExtractorKinds) != 1 || info.ExtractorKinds[0] != KindUnknown {
		t.Fatalf("expected 1 KindUnknown extractor kind, got %v", info.ExtractorKinds)
	}
}

// TestOpenAPI_FilesNotMisclassifiedAsFile pins D8: classification keys off the
// actual extractor base types, so FilesExtractor is files (not file), and a
// user type named with a "File"/"Files" prefix is not mis-classified.
func TestOpenAPI_FilesNotMisclassifiedAsFile(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want ExtractorKind
	}{
		{"FileExtractor", reflect.TypeFor[extractor.FileExtractor](), KindFile},
		{"FilesExtractor", reflect.TypeFor[extractor.FilesExtractor](), KindFiles},
		{"PathExtractor", reflect.TypeFor[extractor.PathExtractor[auditReq]](), KindPath},
		{"QueryExtractor", reflect.TypeFor[extractor.QueryExtractor[auditReq]](), KindQuery},
		{"FormExtractor", reflect.TypeFor[extractor.FormExtractor[auditReq]](), KindForm},
		{"MultipartExtractor", reflect.TypeFor[extractor.MultipartExtractor[auditReq]](), KindMultipart},
		{"HeaderExtractor", reflect.TypeFor[extractor.HeaderExtractor[auditReq]](), KindHeader},
		{"CookieExtractor", reflect.TypeFor[extractor.CookieExtractor[auditReq]](), KindCookie},
		// Pointer form mirrors how handlers take extractors (*Path[T] etc.).
		{"*FilesExtractor", reflect.TypeFor[*extractor.FilesExtractor](), KindFiles},
		// A user type whose name starts with "Files" must not borrow the files kind.
		{"FilesLikeUserType", reflect.TypeFor[filesLikeUserType](), KindUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getExtractorKind(tc.typ); got != tc.want {
				t.Errorf("getExtractorKind(%s) = %q, want %q", tc.typ, got, tc.want)
			}
		})
	}
}

// filesLikeUserType has a "Files"-prefixed name but is not a framework
// extractor; the old prefix table would have mis-classified it as files.
type filesLikeUserType struct {
	Files []string
}

// createdResponse declares its documented status via the statusCoder interface.
type createdResponse struct{}

func (createdResponse) OpenAPIStatusCode() int { return http.StatusCreated }

// TestOpenAPI_RealStatusCode pins D3 at the unit level: extractStatusCode never
// returns 0 (the documented default is 200), and a type that declares its own
// status via statusCoder is documented under that status (201), not 200. The
// register-helper end-to-end behavior is asserted in the root package
// (TestOpenAPI_RealStatusCodeDocumented).
func TestOpenAPI_RealStatusCode(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want int
	}{
		{"string", reflect.TypeFor[string](), http.StatusOK},
		{"struct", reflect.TypeFor[struct{ A int }](), http.StatusOK},
		{"int", reflect.TypeFor[int](), http.StatusOK},
		{"statusCoder", reflect.TypeFor[createdResponse](), http.StatusCreated},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractStatusCode(tc.typ); got != tc.want {
				t.Errorf("extractStatusCode(%s) = %d, want %d", tc.typ, got, tc.want)
			}
		})
	}
}
