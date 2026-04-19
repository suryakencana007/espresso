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
- [x] Implement `espresso.SSE` response type
- [x] Support streaming responses
- [x] Add `SSEWriter` helper for real-time events
- [x] Add tests for SSE
- [x] Document SSE usage

**Use case:** Real-time notifications, live updates, dashboards

---

### 4. Authentication Middleware
- [x] Implement `JWTMiddleware` with configurable claims
- [x] Implement `BasicAuthMiddleware` 
- [x] Implement `APIKeyMiddleware`
- [x] Add tests for all auth middleware (16 tests)
- [x] Document security best practices

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
- [x] Parse handler function signatures
- [x] Generate OpenAPI 3.0 spec from routes
- [x] Support request/response schema generation
- [x] Add `/swagger.json` endpoint via Handler()
- [x] Add Scalar UI integration
- [x] Document OpenAPI generation

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
- [x] Add benchmark suite comparing with gin, echo, fiber (`bench/` module)
- [x] Benchmark handler types
- [x] Benchmark extractors
- [x] Add results to README

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
- [x] Add `extractor.Cookie` documentation to `docs/api/extractor.md`
- [x] Add file upload example to `docs/examples/`
- [x] Add SSE streaming example to `docs/examples/`
- [x] Add authentication example to `docs/examples/`

---

## Test Coverage
Current: **77.8%** | Target: **78%+**

- [ ] Add extractor tests to improve coverage (currently 58.6%)
- [ ] Add middleware edge case tests
- [ ] Add integration tests for complex scenarios

---

## Completed Items

### v1.4.0 (Current)

- [x] Struct-tag validator package (`validator/`) — TODOS #7 closed
- [x] Framework benchmarks vs gin/echo/fiber (`bench/`) — TODOS #10 closed
- [x] Streaming concurrency hardening (race fix, registry leak, readLoop
  guard, serveStream unified)
- [x] Structured JSON error responses for extractor failures
- [x] Handler-cache growth documentation
- [x] v2.0 roadmap (`roadmaps/v2.0/`)

### v1.3.0
- [x] WebSocket handler support (espresso.WS, WebSocket[T])
- [x] Typed SSE streaming (espresso.SSEStream, Stream[T])
- [x] Structured error responses (*espresso.Error, ErrBadRequest, etc.)
- [x] Graceful shutdown hooks (router.OnShutdown, BrewContext)
- [x] Context cancellation tests
- [x] Integration tests for long-lived connections

### v1.2.0
- [x] Fluent API for OpenAPI (New, Description, Server, Schema)
- [x] Handler introspection for automatic type detection
- [x] Operation options (Tags, Summary, Description, Security)
- [x] OpenAPIRouter for seamless routing + documentation
- [x] ServeOpenAPI, ServeDocs methods
- [x] Comprehensive test coverage (104 new tests)

### v1.1.0
- [x] Cookie extractor
- [x] File upload extractors (File, Files, Multipart)
- [x] SSE streaming response
- [x] Authentication middleware (JWT, BasicAuth, APIKey)
- [x] OpenAPI 3.0 generator (initial release)
- [x] Scalar UI for API documentation

### v1.0.2
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