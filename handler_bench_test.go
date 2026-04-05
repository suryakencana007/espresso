package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// BenchmarkHandler_RegistrationWithCache benchmarks handler registration with caching enabled.
func BenchmarkHandler_RegistrationWithCache(b *testing.B) {
	type TestReq struct {
		Name string `json:"name"`
	}

	handler := func(ctx context.Context, req *JSON[TestReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "test"}}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Handler(handler)
	}
}

// BenchmarkHandler_Execution benchmarks handler execution (request processing).
func BenchmarkHandler_Execution(b *testing.B) {
	type TestReq struct {
		Name string `json:"name"`
	}

	handler := func(ctx context.Context, req *JSON[TestReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "test"}}, nil
	}

	httpHandler := Handler(handler)
	reqBody := strings.NewReader(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", reqBody)
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		_, _ = reqBody.Seek(0, 0) // Reset reader for each iteration
		httpHandler(rec, req)
	}
}

// BenchmarkHandler_MultipleRegistrations benchmarks registering the same type multiple times.
func BenchmarkHandler_MultipleRegistrations(b *testing.B) {
	type TestReq struct {
		Name string `json:"name"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler := func(ctx context.Context, req *JSON[TestReq]) (JSON[UserRes], error) {
			return JSON[UserRes]{Data: UserRes{Message: "test"}}, nil
		}
		_ = Handler(handler)
	}
}

// BenchmarkHandler_DifferentTypes benchmarks registering different handler types.
func BenchmarkHandler_DifferentTypes(b *testing.B) {
	types := []struct {
		name string
		fn   any
	}{
		{"Doppio", func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
			return JSON[UserRes]{}, nil
		}},
		{"Solo", func(req *JSON[CreateUserReq]) (JSON[UserRes], error) {
			return JSON[UserRes]{}, nil
		}},
		{"Ristretto", func() Text {
			return Text{Body: "OK"}
		}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, typ := range types {
			_ = Handler(typ.fn)
		}
	}
}
