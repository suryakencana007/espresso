# Espresso TODOs

Priority-sorted task list for Espresso framework development.

---

## High Priority (Core Functionality)

### 1. Cookie Extractor
- [x] Implement `extractor.Cookie[T]` for cookie handling
- [x] Add struct tag support: `cookie:"name,required"`
- [x] Add tests for Cookie extractor
- [x] Document in README and docs

**Use case:** Session tokens, preferences, authentication cookies

---

### 2. Multipart/File Upload Extractor
- [x] Implement `extractor.Multipart[T]` for file uploads
- [x] Support `multipart/form-data` parsing
- [x] Add `extractor.File` type for file metadata
- [x] Add tests for Multipart extractor
- [x] Document file upload examples

**Use case:** Profile pictures, documents, CSV imports

---

### 3. Server-Sent Events (SSE) Response
- [ ] Implement `espresso.SSE` response type
- [ ] Support streaming responses
- [ ] Add `SSEWriter` helper for real-time events
- [ ] Add tests for SSE
- [ ] Document SSE usage

**Use case:** Real-time notifications, live updates, dashboards

---

### 4. Authentication Middleware
- [ ] Implement `JWTMiddleware` with configurable claims
- [ ] Implement `BasicAuthMiddleware` 
- [ ] Implement `APIKeyMiddleware`
- [ ] Add `extractor.Auth[T]` for authenticated user extraction
- [ ] Add tests for all auth middleware
- [ ] Document security best practices

**Use case:** Protected routes, user authentication

---

## Medium Priority (Developer Experience)

### 5. WebSocket Support
- [ ] Add `WebSocketHandler` interface
- [ ] Implement WebSocket upgrader
- [ ] Add connection pool/manager
- [ ] Add tests for WebSocket
- [ ] Document WebSocket usage

**Use case:** Real-time chat, live collaboration, gaming

---

### 6. OpenAPI/Swagger Generator
- [ ] Parse handler function signatures
- [ ] Generate OpenAPI 3.0 spec from routes
- [ ] Support request/response schema generation
- [ ] Add `/swagger.json` endpoint
- [ ] Add Swagger UI integration
- [ ] Document OpenAPI generation

**Use case:** API documentation, client SDK generation

---

### 7. Request Validation Improvements
- [ ] Add `validate` struct tag support
- [ ] Implement validation layer with error messages
- [ ] Support custom validators
- [ ] Add built-in validators (email, url, min, max, regex)
- [ ] Add tests for validation
- [ ] Document validation usage

**Use case:** Input validation before handler execution

---

## Low Priority (Nice to Have)

### 8. GraphQL Adapter
- [ ] Optional `espresso/graphql` package
- [ ] Handler adapter for graphql-go
- [ ] Document GraphQL integration

---

### 9. gRPC Gateway
- [ ] Optional `espresso/grpc` package
- [ ] gRPC-to-HTTP bridge
- [ ] Document gRPC usage

---

### 10. Performance Benchmarks
- [ ] Add benchmark suite comparing with gin, echo, fiber
- [ ] Benchmark handler types
- [ ] Benchmark extractors
- [ ] Add results to README

---

## Code Quality (Technical Debt)

### Critical
- [x] Fix `godot` lint: `handler_test.go:550` - add period to comment
- [x] Fix gosec permissions in `gen-api-docs.go` (G301, G306) - removed file

### Cleanup
- [x] **Remove** `scripts/gen-api-docs.go` - unused, would overwrite manual docs
- [x] Remove `docs:gen-api` script from `package.json`
- [x] Remove reference in `docs/README.md` for `docs:gen-api`

---

## Documentation
- [ ] Add `extractor.Cookie` documentation to `docs/api/extractor.md`
- [ ] Add file upload example to `docs/examples/`
- [ ] Add SSE streaming example to `docs/examples/`
- [ ] Add authentication guide to `docs/guide/`

---

## Test Coverage
Current: **77.8%** | Target: **78%+**

- [ ] Add extractor tests to improve coverage (currently 58.6%)
- [ ] Add middleware edge case tests
- [ ] Add integration tests for complex scenarios

---

## Completed Items

### v1.0.2 (Current)
- [x] Lungo handler for dual extractors
- [x] Handler aliases: Ristretto, Solo, Doppio, Lungo
- [x] State management: WithState, GetState, MustGetState
- [x] VitePress documentation site
- [x] Coffee-themed naming convention

### v1.0.0
- [x] Core router with fluent API
- [x] Extractors: JSON, Query, Path, Header, Form, XML, RawBody
- [x] HTTP middleware: RequestID, Recover, CORS, Compress, RateLimit, Logging
- [x] Service layers: Timeout, Retry, CircuitBreaker, Validation, Logging, Metrics
- [x] Object pooling: BufferPool, ByteSlicePool, StringSlicePool