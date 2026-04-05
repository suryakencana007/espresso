package espresso

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

func TestJSON_WriteResponse(t *testing.T) {
	tests := []struct {
		name       string
		data       any
		statusCode int
		wantStatus int
		wantBody   string
	}{
		{
			name:       "basic JSON response",
			data:       map[string]string{"message": "hello"},
			statusCode: 0,
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
		{
			name:       "JSON with custom status",
			data:       map[string]string{"id": "123"},
			statusCode: http.StatusCreated,
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":"123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			switch d := tt.data.(type) {
			case map[string]string:
				res := JSON[map[string]string]{Data: d, StatusCode: tt.statusCode}
				if err := res.WriteResponse(rec); err != nil {
					t.Errorf("WriteResponse() error = %v", err)
				}
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("WriteResponse() status = %v, want %v", rec.Code, tt.wantStatus)
			}

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("WriteResponse() Content-Type = %v, want application/json", ct)
			}
		})
	}
}

func TestJSON_Extract(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, name, email string)
	}{
		{
			name:    "valid JSON extraction",
			body:    `{"name":"john","email":"john@example.com"}`,
			wantErr: false,
			check: func(t *testing.T, name, email string) {
				if name != "john" {
					t.Errorf("expected Name 'john', got '%s'", name)
				}
				if email != "john@example.com" {
					t.Errorf("expected Email 'john@example.com', got '%s'", email)
				}
			},
		},
		{
			name:    "invalid JSON",
			body:    `{"name":"john"`,
			wantErr: true,
		},
		{
			name:    "empty object",
			body:    `{}`,
			wantErr: false,
			check: func(t *testing.T, name, email string) {
				if name != "" {
					t.Errorf("expected empty Name, got '%s'", name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			ext := &JSON[struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}]{}
			err := ext.Extract(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON.Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil {
				tt.check(t, ext.Data.Name, ext.Data.Email)
			}
		})
	}
}

func TestJSON_Reset(t *testing.T) {
	type TestReq struct {
		Name string `json:"name"`
	}

	j := JSON[TestReq]{
		StatusCode: http.StatusCreated,
		Data:       TestReq{Name: "test"},
	}

	j.Reset()

	if j.StatusCode != 0 {
		t.Errorf("expected StatusCode 0 after reset, got %d", j.StatusCode)
	}
	if j.Data.Name != "" {
		t.Errorf("expected empty Data after reset, got '%s'", j.Data.Name)
	}
}

func TestText_WriteResponse(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		statusCode int
		wantStatus int
	}{
		{
			name:       "basic text response",
			body:       "hello world",
			statusCode: 0,
			wantStatus: http.StatusOK,
		},
		{
			name:       "text with custom status",
			body:       "not found",
			statusCode: http.StatusNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "empty text",
			body:       "",
			statusCode: 0,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			res := Text{Body: tt.body, StatusCode: tt.statusCode}

			if err := res.WriteResponse(rec); err != nil {
				t.Errorf("WriteResponse() error = %v", err)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("WriteResponse() status = %v, want %v", rec.Code, tt.wantStatus)
			}

			if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
				t.Errorf("WriteResponse() Content-Type = %v, want text/plain", ct)
			}

			if rec.Body.String() != tt.body {
				t.Errorf("WriteResponse() body = %v, want %v", rec.Body.String(), tt.body)
			}
		})
	}
}

func TestText_Reset(t *testing.T) {
	txt := Text{Body: "test", StatusCode: http.StatusCreated}
	txt.Reset()

	if txt.Body != "" {
		t.Errorf("expected empty Body after reset, got '%s'", txt.Body)
	}
	if txt.StatusCode != 0 {
		t.Errorf("expected StatusCode 0 after reset, got %d", txt.StatusCode)
	}
}

func TestStatus_WriteResponse(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		wantStatus int
	}{
		{
			name:       "status no content",
			status:     Status(http.StatusNoContent),
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "status created",
			status:     Status(http.StatusCreated),
			wantStatus: http.StatusCreated,
		},
		{
			name:       "status accepted",
			status:     Status(http.StatusAccepted),
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			if err := tt.status.WriteResponse(rec); err != nil {
				t.Errorf("WriteResponse() error = %v", err)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("WriteResponse() status = %v, want %v", rec.Code, tt.wantStatus)
			}

			if rec.Body.Len() != 0 {
				t.Errorf("WriteResponse() expected empty body, got %v", rec.Body.String())
			}
		})
	}
}

func TestStatus_Reset(t *testing.T) {
	s := Status(http.StatusCreated)
	s.Reset()

	if s != 0 {
		t.Errorf("expected Status 0 after reset, got %d", s)
	}
}

func TestJSON_Bidirectional(t *testing.T) {
	type UserReq struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	type UserRes struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	t.Run("extract request", func(t *testing.T) {
		body := `{"name":"john","email":"john@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		ext := &JSON[UserReq]{}
		if err := ext.Extract(req); err != nil {
			t.Errorf("Extract() error = %v", err)
		}

		if ext.Data.Name != "john" {
			t.Errorf("expected Name 'john', got '%s'", ext.Data.Name)
		}
		if ext.Data.Email != "john@example.com" {
			t.Errorf("expected Email 'john@example.com', got '%s'", ext.Data.Email)
		}
	})

	t.Run("write response", func(t *testing.T) {
		rec := httptest.NewRecorder()
		res := JSON[UserRes]{
			StatusCode: http.StatusCreated,
			Data: UserRes{
				ID:    1,
				Name:  "john",
				Email: "john@example.com",
			},
		}

		if err := res.WriteResponse(rec); err != nil {
			t.Errorf("WriteResponse() error = %v", err)
		}

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
		}

		var result UserRes
		if err := sonic.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Errorf("json.Unmarshal() error = %v", err)
		}

		if result.ID != 1 {
			t.Errorf("expected ID 1, got %d", result.ID)
		}
	})
}
