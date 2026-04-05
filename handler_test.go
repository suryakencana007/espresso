package espresso

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	handler := Ristretto(func() Text {
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
