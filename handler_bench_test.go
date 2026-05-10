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
		{"Ristretto", func(_ context.Context) Text {
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

// BenchmarkHandlerCache_SteadyState measures the cost of a cache HIT — the
// path every static-route app hits on every registration after the first.
// Single distinct handler type, repeated Loads. Should be sub-100ns:
// just the mutex + map lookup + linked-list move-to-front, no reflection.
func BenchmarkHandlerCache_SteadyState(b *testing.B) {
	cache := newBoundedHandlerCache(DefaultHandlerCacheSize)
	types := makeDistinctTypes(1)
	cache.Store(types[0], &handlerInfo{numIn: 0})

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = cache.Load(types[0])
	}
}

// BenchmarkRunDefaultValidator_NilHook measures the nil-fast path on the
// per-request hot path. With no validator installed (the v1.x default),
// every Extract call ends with this — it should be a single atomic load
// + branch and compile to almost nothing.
func BenchmarkRunDefaultValidator_NilHook(b *testing.B) {
	v := struct{ Name string }{Name: "x"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = RunDefaultValidator(&v)
	}
}

// BenchmarkRunDefaultValidator_HookSet measures the cost when a validator
// IS installed. Difference vs. the nil-hook bench is the call-through
// overhead.
func BenchmarkRunDefaultValidator_HookSet(b *testing.B) {
	SetDefaultValidator(func(any) error { return nil })
	b.Cleanup(func() { SetDefaultValidator(nil) })

	v := struct{ Name string }{Name: "x"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = RunDefaultValidator(&v)
	}
}

// BenchmarkHandlerCache_Overflow measures the cost of registering N handler
// types into a smaller cache, characterizing eviction overhead. Each Store
// past capacity evicts one entry; the bench reports per-Store cost
// (insertion + LRU update + eviction + onEvict hook fire).
func BenchmarkHandlerCache_Overflow(b *testing.B) {
	const cap = 64
	const overflow = 256
	types := makeDistinctTypes(overflow)

	b.ReportAllocs()
	b.ResetTimer()
	for n := range b.N {
		// Reset cache each batch to keep the eviction pattern uniform.
		// Cost-per-iteration is amortized across overflow inserts.
		if n%overflow == 0 {
			b.StopTimer()
			cache := newBoundedHandlerCache(cap)
			b.StartTimer()
			for _, t := range types {
				cache.Store(t, &handlerInfo{numIn: 0})
			}
		}
	}
}
