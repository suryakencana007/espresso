package espresso

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRequest(body io.Reader) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/test", body)
}

func TestDecodeSafeJSON_ValidJSON(t *testing.T) {
	body := `{"name":"john","age":30}`
	req := newTestRequest(strings.NewReader(body))

	var v struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := DecodeSafeJSON(req, &v)
	if err != nil {
		t.Errorf("DecodeSafeJSON() error = %v", err)
	}

	if v.Name != "john" {
		t.Errorf("expected Name 'john', got '%s'", v.Name)
	}
	if v.Age != 30 {
		t.Errorf("expected Age 30, got %d", v.Age)
	}
}

func TestDecodeSafeJSON_InvalidJSON(t *testing.T) {
	body := `{"name":"john"`
	req := newTestRequest(strings.NewReader(body))

	var v struct {
		Name string `json:"name"`
	}

	err := DecodeSafeJSON(req, &v)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeSafeJSON_EmptyObject(t *testing.T) {
	body := `{}`
	req := newTestRequest(strings.NewReader(body))

	var v struct {
		Name string `json:"name"`
	}

	err := DecodeSafeJSON(req, &v)
	if err != nil {
		t.Errorf("DecodeSafeJSON() error = %v", err)
	}

	if v.Name != "" {
		t.Errorf("expected empty Name, got '%s'", v.Name)
	}
}

func TestDecodeSafeJSON_Array(t *testing.T) {
	body := `[1, 2, 3]`
	req := newTestRequest(strings.NewReader(body))

	var v []int
	err := DecodeSafeJSON(req, &v)
	if err != nil {
		t.Errorf("DecodeSafeJSON() error = %v", err)
	}

	if len(v) != 3 {
		t.Errorf("expected 3 elements, got %d", len(v))
	}
}

func TestDecodeSafeJSON_LargePayload(t *testing.T) {
	largeBody := bytes.Repeat([]byte(`{"name":"x"}`), 200000)
	req := newTestRequest(bytes.NewReader(largeBody))

	var v struct {
		Name string `json:"name"`
	}

	err := DecodeSafeJSON(req, &v)
	if err == nil {
		t.Error("expected error for large payload, got nil")
	}
}

func TestDecodeSafeJSON_NestedStruct(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	body := `{"name":"outer","inner":{"value":"inner_value"}}`
	req := newTestRequest(strings.NewReader(body))

	var v Outer
	err := DecodeSafeJSON(req, &v)
	if err != nil {
		t.Errorf("DecodeSafeJSON() error = %v", err)
	}

	if v.Name != "outer" {
		t.Errorf("expected Name 'outer', got '%s'", v.Name)
	}
	if v.Inner.Value != "inner_value" {
		t.Errorf("expected Inner.Value 'inner_value', got '%s'", v.Inner.Value)
	}
}

func TestBufferPool(t *testing.T) {
	buf1 := getBuffer()
	if buf1 == nil {
		t.Error("expected non-nil buffer from pool")
	}

	buf1.WriteString("test data")
	if buf1.String() != "test data" {
		t.Errorf("expected 'test data', got '%s'", buf1.String())
	}

	putBuffer(buf1)

	buf1.Reset()

	if buf1.Len() != 0 {
		t.Error("expected buffer to be reset after putBuffer")
	}
}

func TestBufferPool_LargeBuffer(t *testing.T) {
	largeBuf := make([]byte, MaxPoolSize+1024)
	buf := bytes.NewBuffer(largeBuf)

	putBuffer(buf)
}

func TestBufferPool_Multiple(t *testing.T) {
	for i := 0; i < 10; i++ {
		buf := getBuffer()
		buf.WriteString("test")
		putBuffer(buf)
	}
}

func TestMaxPayloadSize(t *testing.T) {
	if MaxPayloadSize != 1*1024*1024 {
		t.Errorf("expected MaxPayloadSize %d, got %d", 1*1024*1024, MaxPayloadSize)
	}
}

func TestMaxPoolSize(t *testing.T) {
	if MaxPoolSize != 64*1024 {
		t.Errorf("expected MaxPoolSize %d, got %d", 64*1024, MaxPoolSize)
	}
}

func TestDecodeSafeJSON_MultipleCalls(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	for i := 0; i < 5; i++ {
		body := `{"name":"test` + string(rune('0'+i)) + `"}`
		req := newTestRequest(strings.NewReader(body))

		var v TestStruct
		err := DecodeSafeJSON(req, &v)
		if err != nil {
			t.Errorf("DecodeSafeJSON() error = %v", err)
		}
	}
}

func TestDecodeSafeJSON_ContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     bool
	}{
		{
			name:        "application/json",
			contentType: "application/json",
			body:        `{"name":"test"}`,
			wantErr:     false,
		},
		{
			name:        "text/plain",
			contentType: "text/plain",
			body:        `{"name":"test"}`,
			wantErr:     false,
		},
		{
			name:        "no content type",
			contentType: "",
			body:        `{"name":"test"}`,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest(strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			var v struct {
				Name string `json:"name"`
			}

			err := DecodeSafeJSON(req, &v)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeSafeJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkDecodeSafeJSON(b *testing.B) {
	body := `{"name":"john","email":"john@example.com"}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := newTestRequest(strings.NewReader(body))
		var v struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		_ = DecodeSafeJSON(req, &v)
	}
}

func BenchmarkBufferPool(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := getBuffer()
		buf.WriteString("test data")
		putBuffer(buf)
	}
}
