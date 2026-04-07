package httpmiddleware

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// JWTConfig defines configuration for JWT middleware.
type JWTConfig struct {
	Secret          string
	SigningMethod   string // HS256, HS384, HS512, RS256, RS384, RS512
	TokenLookup     string // "header:Authorization" or "query:token" or "cookie:jwt"
	TokenHeader     string // "Bearer"
	ContextKey      string // Key to store claims in context
	Skipper         func(r *http.Request) bool
	ClaimsExtractor func(token string) (map[string]any, error)
}

// DefaultJWTConfig is the default JWT configuration.
var DefaultJWTConfig = JWTConfig{
	SigningMethod: "HS256",
	TokenLookup:   "header:Authorization",
	TokenHeader:   "Bearer",
	ContextKey:    "user",
}

// JWTMiddleware validates JWT tokens and stores claims in context.
func JWTMiddleware(config JWTConfig) Middleware {
	if config.Secret == "" {
		panic("jwt: secret is required")
	}
	if config.SigningMethod == "" {
		config.SigningMethod = DefaultJWTConfig.SigningMethod
	}
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultJWTConfig.TokenLookup
	}
	if config.TokenHeader == "" {
		config.TokenHeader = DefaultJWTConfig.TokenHeader
	}
	if config.ContextKey == "" {
		config.ContextKey = DefaultJWTConfig.ContextKey
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.Skipper != nil && config.Skipper(r) {
				next.ServeHTTP(w, r)
				return
			}

			token, err := extractToken(r, config.TokenLookup, config.TokenHeader)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if config.ClaimsExtractor == nil {
				http.Error(w, "Unauthorized: no claims extractor", http.StatusUnauthorized)
				return
			}

			claims, err := config.ClaimsExtractor(token)
			if err != nil {
				http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey(config.ContextKey), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BasicAuthConfig defines configuration for Basic Auth middleware.
type BasicAuthConfig struct {
	Realm     string
	Users     map[string]string // username -> password
	Skipper   func(r *http.Request) bool
	Validator func(username, password string) bool
}

// DefaultBasicAuthConfig is the default Basic Auth configuration.
var DefaultBasicAuthConfig = BasicAuthConfig{
	Realm: "Restricted",
}

// BasicAuthMiddleware validates Basic Auth credentials.
func BasicAuthMiddleware(config BasicAuthConfig) Middleware {
	if config.Realm == "" {
		config.Realm = DefaultBasicAuthConfig.Realm
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.Skipper != nil && config.Skipper(r) {
				next.ServeHTTP(w, r)
				return
			}

			username, password, ok := extractBasicAuth(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if config.Validator != nil {
				if !config.Validator(username, password) {
					w.Header().Set("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			} else {
				if !validateBasicAuth(username, password, config.Users) {
					w.Header().Set("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), AuthKey{}, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyConfig defines configuration for API Key middleware.
type APIKeyConfig struct {
	Keys         []string
	KeyLookup    string // "header:X-API-Key" or "query:api_key" or "cookie:api_key"
	ContextKey   string
	Skipper      func(r *http.Request) bool
	KeyValidator func(key string) bool
}

// DefaultAPIKeyConfig is the default API Key configuration.
var DefaultAPIKeyConfig = APIKeyConfig{
	KeyLookup:  "header:X-API-Key",
	ContextKey: "api_key",
}

// APIKeyMiddleware validates API keys in requests.
func APIKeyMiddleware(config APIKeyConfig) Middleware {
	if config.KeyLookup == "" {
		config.KeyLookup = DefaultAPIKeyConfig.KeyLookup
	}
	if config.ContextKey == "" {
		config.ContextKey = DefaultAPIKeyConfig.ContextKey
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.Skipper != nil && config.Skipper(r) {
				next.ServeHTTP(w, r)
				return
			}

			key, err := extractAPIKey(r, config.KeyLookup)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if config.KeyValidator != nil {
				if !config.KeyValidator(key) {
					http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
					return
				}
			} else {
				if !validateAPIKey(key, config.Keys) {
					http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), contextKey(config.ContextKey), key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper functions

func contextKey(key string) any {
	return string(contextKeyPrefix) + key
}

type contextKeyType string

const contextKeyPrefix contextKeyType = "auth_"

func extractToken(r *http.Request, lookup, header string) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		return "", ErrInvalidTokenLookup
	}

	switch parts[0] {
	case "header":
		auth := r.Header.Get(parts[1])
		if auth == "" {
			return "", ErrNoToken
		}
		if header != "" {
			if !strings.HasPrefix(auth, header+" ") {
				return "", ErrInvalidTokenFormat
			}
			return strings.TrimPrefix(auth, header+" "), nil
		}
		return auth, nil
	case "query":
		token := r.URL.Query().Get(parts[1])
		if token == "" {
			return "", ErrNoToken
		}
		return token, nil
	case "cookie":
		cookie, err := r.Cookie(parts[1])
		if err != nil {
			return "", ErrNoToken
		}
		return cookie.Value, nil
	default:
		return "", ErrInvalidTokenLookup
	}
}

func extractBasicAuth(r *http.Request) (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", "", false
	}

	if !strings.HasPrefix(auth, "Basic ") {
		return "", "", false
	}

	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		return "", "", false
	}

	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return "", "", false
	}

	return pair[0], pair[1], true
}

func validateBasicAuth(username, password string, users map[string]string) bool {
	if users == nil {
		return false
	}
	expectedPassword, exists := users[username]
	if !exists {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}

func extractAPIKey(r *http.Request, lookup string) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		return "", ErrInvalidKeyLookup
	}

	switch parts[0] {
	case "header":
		key := r.Header.Get(parts[1])
		if key == "" {
			return "", ErrNoAPIKey
		}
		return key, nil
	case "query":
		key := r.URL.Query().Get(parts[1])
		if key == "" {
			return "", ErrNoAPIKey
		}
		return key, nil
	case "cookie":
		cookie, err := r.Cookie(parts[1])
		if err != nil {
			return "", ErrNoAPIKey
		}
		return cookie.Value, nil
	default:
		return "", ErrInvalidKeyLookup
	}
}

func validateAPIKey(key string, keys []string) bool {
	for _, k := range keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(k)) == 1 {
			return true
		}
	}
	return false
}

// GetClaims retrieves JWT claims from context.
func GetClaims(ctx context.Context, key string) map[string]any {
	claims, _ := ctx.Value(contextKey(key)).(map[string]any)
	return claims
}

// GetUsername retrieves username from context (Basic Auth).
func GetUsername(ctx context.Context) string {
	username, _ := ctx.Value(AuthKey{}).(string)
	return username
}

// GetAPIKey retrieves API key from context.
func GetAPIKey(ctx context.Context, key string) string {
	apiKey, _ := ctx.Value(contextKey(key)).(string)
	return apiKey
}

// Auth errors.
var (
	ErrInvalidTokenLookup = &AuthError{Code: "invalid_token_lookup", Message: "invalid token lookup configuration"}
	ErrNoToken            = &AuthError{Code: "no_token", Message: "no token found"}
	ErrInvalidTokenFormat = &AuthError{Code: "invalid_token_format", Message: "invalid token format"}
	ErrInvalidKeyLookup   = &AuthError{Code: "invalid_key_lookup", Message: "invalid API key lookup configuration"}
	ErrNoAPIKey           = &AuthError{Code: "no_api_key", Message: "no API key found"}
)

// AuthError represents an authentication error.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
