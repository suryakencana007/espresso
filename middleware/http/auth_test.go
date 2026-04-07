package httpmiddleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTMiddleware_ValidToken(t *testing.T) {
	config := JWTConfig{
		Secret: "test-secret",
		ClaimsExtractor: func(token string) (map[string]any, error) {
			return map[string]any{"sub": "user123", "role": "admin"}, nil
		},
	}

	middleware := JWTMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context(), "user")
		if claims == nil {
			t.Error("expected claims to be present")
		}
		if claims["sub"] != "user123" {
			t.Errorf("expected sub 'user123', got '%v'", claims["sub"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	config := JWTConfig{
		Secret: "test-secret",
		ClaimsExtractor: func(token string) (map[string]any, error) {
			return map[string]any{"sub": "user123"}, nil
		},
	}

	middleware := JWTMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	config := JWTConfig{
		Secret: "test-secret",
		ClaimsExtractor: func(token string) (map[string]any, error) {
			return nil, ErrNoToken
		},
	}

	middleware := JWTMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestJWTMiddleware_QueryToken(t *testing.T) {
	config := JWTConfig{
		Secret:      "test-secret",
		TokenLookup: "query:token",
		ClaimsExtractor: func(token string) (map[string]any, error) {
			return map[string]any{"sub": "user123"}, nil
		},
	}

	middleware := JWTMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context(), "user")
		if claims["sub"] != "user123" {
			t.Errorf("expected sub 'user123', got '%v'", claims["sub"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?token=mytoken", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestJWTMiddleware_Skipper(t *testing.T) {
	config := JWTConfig{
		Secret: "test-secret",
		Skipper: func(r *http.Request) bool {
			return r.URL.Path == "/health"
		},
		ClaimsExtractor: func(token string) (map[string]any, error) {
			return nil, ErrNoToken
		},
	}

	middleware := JWTMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestBasicAuthMiddleware_ValidCredentials(t *testing.T) {
	config := BasicAuthConfig{
		Realm: "Test",
		Users: map[string]string{
			"admin": "password123",
		},
	}

	middleware := BasicAuthMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r.Context())
		if username != "admin" {
			t.Errorf("expected username 'admin', got '%s'", username)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "password123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestBasicAuthMiddleware_InvalidCredentials(t *testing.T) {
	config := BasicAuthConfig{
		Realm: "Test",
		Users: map[string]string{
			"admin": "password123",
		},
	}

	middleware := BasicAuthMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "wrong-password")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestBasicAuthMiddleware_MissingAuth(t *testing.T) {
	config := BasicAuthConfig{
		Realm: "Test",
		Users: map[string]string{
			"admin": "password123",
		},
	}

	middleware := BasicAuthMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if !contains(rec.Header().Get("WWW-Authenticate"), "Basic realm") {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestBasicAuthMiddleware_Validator(t *testing.T) {
	config := BasicAuthConfig{
		Realm: "Test",
		Validator: func(username, password string) bool {
			return username == "test" && password == "test123"
		},
	}

	middleware := BasicAuthMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("test", "test123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestBasicAuthMiddleware_Skipper(t *testing.T) {
	config := BasicAuthConfig{
		Realm: "Test",
		Users: map[string]string{"admin": "password"},
		Skipper: func(r *http.Request) bool {
			return r.URL.Path == "/public"
		},
	}

	middleware := BasicAuthMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	config := APIKeyConfig{
		Keys:       []string{"key123", "key456"},
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := GetAPIKey(r.Context(), "api_key")
		if key != "key123" {
			t.Errorf("expected key 'key123', got '%s'", key)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "key123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	config := APIKeyConfig{
		Keys:      []string{"key123", "key456"},
		KeyLookup: "header:X-API-Key",
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAPIKeyMiddleware_MissingKey(t *testing.T) {
	config := APIKeyConfig{
		Keys:      []string{"key123"},
		KeyLookup: "header:X-API-Key",
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAPIKeyMiddleware_QueryKey(t *testing.T) {
	config := APIKeyConfig{
		Keys:      []string{"key123"},
		KeyLookup: "query:api_key",
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?api_key=key123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAPIKeyMiddleware_Validator(t *testing.T) {
	config := APIKeyConfig{
		KeyLookup: "header:X-API-Key",
		KeyValidator: func(key string) bool {
			return key == "valid-key"
		},
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAPIKeyMiddleware_Skipper(t *testing.T) {
	config := APIKeyConfig{
		Keys: []string{"key123"},
		Skipper: func(r *http.Request) bool {
			return r.URL.Path == "/health"
		},
	}

	middleware := APIKeyMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuthError(t *testing.T) {
	err := ErrNoToken
	if err.Code != "no_token" {
		t.Errorf("expected code 'no_token', got '%s'", err.Code)
	}
	if err.Error() != "no token found" {
		t.Errorf("expected message 'no token found', got '%s'", err.Error())
	}
}

func TestGetClaims_Nil(t *testing.T) {
	ctx := context.Background()
	claims := GetClaims(ctx, "user")
	if claims != nil {
		t.Error("expected nil claims for empty context")
	}
}

func TestGetUsername_Empty(t *testing.T) {
	ctx := context.Background()
	username := GetUsername(ctx)
	if username != "" {
		t.Errorf("expected empty username, got '%s'", username)
	}
}

func TestGetAPIKey_Empty(t *testing.T) {
	ctx := context.Background()
	key := GetAPIKey(ctx, "api_key")
	if key != "" {
		t.Errorf("expected empty key, got '%s'", key)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
