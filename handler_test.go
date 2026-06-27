package espresso

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

type testReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (r *testReq) Extract(req *http.Request) error {
	return sonic.ConfigDefault.NewDecoder(req.Body).Decode(r)
}

func (r *testReq) Reset() {
	r.Name = ""
	r.Email = ""
}

type testRes struct {
	Message string `json:"message"`
}

func (r testRes) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	return sonic.ConfigDefault.NewEncoder(w).Encode(r)
}

func TestHandler_FuncNoArgs(t *testing.T) {
	handler := Handler(func() testRes {
		return testRes{Message: "hello"}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "hello" {
		t.Errorf("expected message 'hello', got '%s'", result.Message)
	}
}

func TestHandler_FuncNoArgs_Error(t *testing.T) {
	handler := Handler(func() (testRes, error) {
		return testRes{Message: "success"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandler_FuncReqArg(t *testing.T) {
	handler := Handler(func(req *testReq) testRes {
		return testRes{Message: "hello " + req.Name}
	})

	body := `{"name":"john","email":"john@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "hello john" {
		t.Errorf("expected message 'hello john', got '%s'", result.Message)
	}
}

func TestHandler_FuncReqArg_Error(t *testing.T) {
	handler := Handler(func(req *testReq) (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	body := `{"name":"john","email":"john@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandler_FuncCtxReqErr(t *testing.T) {
	handler := Handler(func(ctx context.Context, req *testReq) (testRes, error) {
		return testRes{Message: "hello " + req.Name}, nil
	})

	body := `{"name":"john","email":"john@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "hello john" {
		t.Errorf("expected message 'hello john', got '%s'", result.Message)
	}
}

func TestHandler_HttpHandlerFunc(t *testing.T) {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("direct handler"))
	})

	handler := Handler(hf)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Body.String() != "direct handler" {
		t.Errorf("expected body 'direct handler', got '%s'", rec.Body.String())
	}
}

func TestHandler_HttpHandler(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("http handler"))
	})

	handler := Handler(h)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestRistretto(t *testing.T) {
	handler := Ristretto(func(_ context.Context) Text {
		return Text{Body: "pong"}
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Body.String() != "pong" {
		t.Errorf("expected body 'pong', got '%s'", rec.Body.String())
	}
}

func TestSolo(t *testing.T) {
	handler := Solo(func(req *testReq) (testRes, error) {
		return testRes{Message: "hello " + req.Name}, nil
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDoppio(t *testing.T) {
	handler := Doppio(func(ctx context.Context, req *testReq) (testRes, error) {
		return testRes{Message: "hello " + req.Name}, nil
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDoppio_InvalidJSON(t *testing.T) {
	handler := Doppio(func(ctx context.Context, req *testReq) (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

type testService struct{}

func (s testService) Call(ctx context.Context, req *testReq) (testRes, error) {
	return testRes{Message: "service " + req.Name}, nil
}

func TestHandler_Service(t *testing.T) {
	handler := Handler(testService{})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "service john" {
		t.Errorf("expected message 'service john', got '%s'", result.Message)
	}
}

func TestHandler_ErrorResponse(t *testing.T) {
	handler := Handler(func(ctx context.Context, req *testReq) (testRes, error) {
		return testRes{}, &testError{msg: "something went wrong"}
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestHandler_PanicInvalidSignature(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid handler signature")
		}
	}()

	Handler(func(a int, b string) {})
}

func TestHandler_RequestBodyClosed(t *testing.T) {
	body := []byte(`{"name":"john"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	handler := Handler(func(req *testReq) testRes {
		return testRes{Message: "ok"}
	})

	rec := httptest.NewRecorder()
	handler(rec, req)

	_, err := io.ReadAll(req.Body)
	if err == nil {
		t.Log("body already closed or empty")
	}
}

func TestHandlerCtx(t *testing.T) {
	handler := HandlerCtx(func(ctx context.Context) (testRes, error) {
		return testRes{Message: "context ok"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerCtxNoErr(t *testing.T) {
	handler := HandlerCtxNoErr(func(ctx context.Context) testRes {
		return testRes{Message: "no error"}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerCtxReq(t *testing.T) {
	handler := HandlerCtxReq(func(ctx context.Context, req *testReq) testRes {
		return testRes{Message: "hello " + req.Name}
	})

	body := `{"name":"world"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerReqErr(t *testing.T) {
	handler := HandlerReqErr(func(req *testReq) (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerReq(t *testing.T) {
	handler := HandlerReq(func(req *testReq) testRes {
		return testRes{Message: "ok"}
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerNoReq(t *testing.T) {
	handler := HandlerNoReq(func() (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerNoReqNoErr(t *testing.T) {
	handler := HandlerNoReqNoErr(func() testRes {
		return testRes{Message: "ok"}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestHandlerCtxReqErr(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		handler := HandlerCtxReqErr(func(ctx context.Context, req *testReq) (testRes, error) {
			return testRes{Message: "hello " + req.Name}, nil
		})

		body := `{"name":"world"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("handler returns error", func(t *testing.T) {
		handler := HandlerCtxReqErr(func(ctx context.Context, req *testReq) (testRes, error) {
			return testRes{}, errors.New("handler error")
		})

		body := `{"name":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

func TestHandlerReqErr_WithNilBody(t *testing.T) {
	handler := HandlerReqErr(func(req *testReq) (testRes, error) {
		return testRes{Message: req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for nil body, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandlerCtx_WithError(t *testing.T) {
	handler := HandlerCtx(func(ctx context.Context) (testRes, error) {
		return testRes{}, errors.New("context error")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestHandlerNoReq_WithError(t *testing.T) {
	handler := HandlerNoReq(func() (testRes, error) {
		return testRes{}, errors.New("no request error")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestHandlerCtxReq_WithError(t *testing.T) {
	handler := HandlerCtxReq(func(ctx context.Context, req *testReq) testRes {
		return testRes{Message: "context request"}
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// Test types for Lungo (two extractors).
type testPathID struct {
	ID int64 `path:"id"`
}

func (r *testPathID) Extract(req *http.Request) error {
	// Simulate path parameter extraction
	idStr := req.PathValue("id")
	if idStr == "" {
		idStr = "123" // Default for tests
	}
	var err error
	r.ID, err = parseInt64(idStr)
	return err
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid id")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

func (r *testPathID) Reset() {
	r.ID = 0
}

func TestLungo(t *testing.T) {
	handler := Lungo(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "path and body ok"}, nil
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "path and body ok" {
		t.Errorf("expected message 'path and body ok', got '%s'", result.Message)
	}
}

func TestLungo_WithValues(t *testing.T) {
	handler := Lungo(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "id=" + strconv.FormatInt(path.ID, 10) + ",name=" + body.Name}, nil
	})

	body := `{"name":"john","email":"john@example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/users/456", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "456")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestLungo_FirstExtractorError(t *testing.T) {
	handler := Lungo(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/invalid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "invalid!")
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Path extraction should fail with invalid ID
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLungo_SecondExtractorError(t *testing.T) {
	handler := Lungo(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "ok"}, nil
	})

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Body extraction should fail with invalid JSON
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLungo_HandlerError(t *testing.T) {
	handler := Lungo(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{}, errors.New("handler error")
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestLungoNoErr(t *testing.T) {
	handler := LungoNoErr(func(ctx context.Context, path *testPathID, body *testReq) testRes {
		return testRes{Message: "no error"}
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result testRes
	if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if result.Message != "no error" {
		t.Errorf("expected message 'no error', got '%s'", result.Message)
	}
}

func TestHandlerCtxReq1Req2Err(t *testing.T) {
	handler := HandlerCtxReq1Req2Err(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "both extractors ok"}, nil
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlerCtxReq1Req2(t *testing.T) {
	handler := HandlerCtxReq1Req2(func(ctx context.Context, path *testPathID, body *testReq) testRes {
		return testRes{Message: "typed handler"}
	})

	body := `{"name":"john"}`
	req := httptest.NewRequest(http.MethodPut, "/users/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestReflectionTwoExtractor asserts that a two-extractor handler registered via
// the reflection path (Handler / router.Get) fails fast at registration (v2.2
// approach B) instead of panicking per-request with "this is a bug".
func TestReflectionTwoExtractor(t *testing.T) {
	twoExtractor := func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{Message: "unreachable"}, nil
	}

	t.Run("Handler panics at registration", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected Handler to panic at registration for two-extractor signature")
			}
		}()
		Handler(twoExtractor)
	})

	t.Run("router.Get panics at registration", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected router.Get to panic at registration for two-extractor signature")
			}
		}()
		r := Portafilter()
		r.Get("/u/{id}", twoExtractor)
	})
}

// TestReflectionTwoExtractor_RegistrationPanicMessage asserts the registration
// panic names the typed alternative so the user knows the fix.
func TestReflectionTwoExtractor_RegistrationPanicMessage(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for two-extractor reflection handler")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic value, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "HandlerCtxReq1Req2Err") {
			t.Errorf("panic message must name HandlerCtxReq1Req2Err, got: %q", msg)
		}
		if !strings.Contains(msg, "Lungo") {
			t.Errorf("panic message must name Lungo, got: %q", msg)
		}
		if !strings.Contains(msg, "FromRequest") {
			t.Errorf("panic message should mention FromRequest arguments, got: %q", msg)
		}
	}()

	Handler(func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
		return testRes{}, nil
	})
}

// TestLungoTwoExtractor_Regression locks the typed two-extractor path: both
// extractors must populate. This guards the "typed path unchanged" criterion.
func TestLungoTwoExtractor_Regression(t *testing.T) {
	for _, name := range []string{"Lungo", "HandlerCtxReq1Req2Err"} {
		t.Run(name, func(t *testing.T) {
			fn := func(ctx context.Context, path *testPathID, body *testReq) (testRes, error) {
				return testRes{Message: "id=" + strconv.FormatInt(path.ID, 10) + ",name=" + body.Name}, nil
			}
			var handler http.HandlerFunc
			if name == "Lungo" {
				handler = Lungo(fn)
			} else {
				handler = HandlerCtxReq1Req2Err(fn)
			}

			body := `{"name":"john","email":"john@example.com"}`
			req := httptest.NewRequest(http.MethodPut, "/users/789", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", "789")
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			var result testRes
			if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("sonic.Unmarshal() error = %v", err)
			}
			if result.Message != "id=789,name=john" {
				t.Errorf("expected both extractors populated, got %q", result.Message)
			}
		})
	}
}

// TestReflectionSingleExtractor_Unaffected confirms zero- and single-extractor
// reflection handlers still register and serve after the two-extractor guard.
func TestReflectionSingleExtractor_Unaffected(t *testing.T) {
	t.Run("func() T", func(t *testing.T) {
		handler := Handler(func() testRes { return testRes{Message: "zero"} })
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/test", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("func(*Req) T", func(t *testing.T) {
		handler := Handler(func(req *testReq) testRes { return testRes{Message: "hello " + req.Name} })
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"jane"}`))
		r.Header.Set("Content-Type", "application/json")
		handler(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		var result testRes
		if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("sonic.Unmarshal() error = %v", err)
		}
		if result.Message != "hello jane" {
			t.Errorf("expected 'hello jane', got %q", result.Message)
		}
	})

	t.Run("func(ctx, *Req) (T, error)", func(t *testing.T) {
		handler := Handler(func(ctx context.Context, req *testReq) (testRes, error) {
			return testRes{Message: "hello " + req.Name}, nil
		})
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"jane"}`))
		r.Header.Set("Content-Type", "application/json")
		handler(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		var result testRes
		if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("sonic.Unmarshal() error = %v", err)
		}
		if result.Message != "hello jane" {
			t.Errorf("expected 'hello jane', got %q", result.Message)
		}
	})
}
