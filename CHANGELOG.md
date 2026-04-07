# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2025-04-08

### Added
- **Cookie extractor** - `extractor.Cookie[T]` for HTTP cookies
- **File upload extractors** - `extractor.File`, `extractor.Files`, `extractor.Multipart` for file uploads
- **SSE streaming** - `response.SSE` for Server-Sent Events support
- **Authentication middleware** - JWT, BasicAuth, APIKey middlewares in `middleware/http`
  - `JWTMiddleware` with RS256/HS256 support
  - `BasicAuthMiddleware` with user validation
  - `APIKeyMiddleware` with header/query param support
- **OpenAPI generator** - Package `openapi` for OpenAPI 3.0 specification generation
  - `NewGenerator()`, `AddPath()`, `AddSchema()`, `Handler()`
  - `GenerateSchemaFromType()` for automatic schema generation from Go types
- **Scalar UI** - Modern API documentation UI (`ScalarUIHandler`, `ScalarUI`)

### Changed
- Replaced `interface{}` with `any` throughout codebase (Go 1.18+ idiom)

### Documentation
- Added comprehensive examples for file upload, SSE streaming, and authentication
- Added authentication middleware documentation

## [1.0.2] - 2025-01-XX

### Added
- **Lungo handler** - New handler for 3-parameter functions (context + 2 extractors)
  - `HandlerCtxReq1Req2Err[Req1, Req2, Res]` - Typed handler for dual extractors
  - `HandlerCtxReq1Req2[Req1, Req2, Res]` - Variant without error return
  - `Lungo[Req1, Req2, Res]` - Coffee-themed alias (named after "long" espresso)
  - `LungoNoErr[Req1, Req2, Res]` - No-error variant
  - Use case: handlers needing both path parameters AND request body

### Fixed
- Escaped angle brackets in Go comments for Vue parsing
- Fixed documentation sidebar link (handler → espresso)
- Fixed ignore dead links in VitePress config
- Updated documentation for bun instead of npm

### Documentation
- Complete VitePress documentation site with guides, examples, and API reference
- Added Mermaid diagram support
- Added comprehensive examples (basic-api, middleware-stack, state-management, production)

## [1.0.1] - 2024-12-XX

### Added
- Initial VitePress documentation site
- Code Hike integration for syntax highlighting
- Mermaid diagram support

## [1.0.0] - 2024-12-XX

### Added
- Initial release
- Core router with fluent API (`Portafilter()`, `Get()`, `Post()`, `Put()`, `Delete()`, etc.)
- Handler aliases: `Ristretto()`, `Solo()`, `Doppio()`
- Built-in response types: `JSON[T]`, `Text`, `Status`
- State/dependency injection with `WithState()` and `GetState[T]()`
- Extractors: `JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`, `Header[T]`, `XML[T]`
- HTTP middleware: `RequestIDMiddleware`, `RecoverMiddleware`, `CORSMiddleware`, `CompressMiddleware`, `RateLimitMiddleware`, `AuthMiddleware`
- Service layers: `LoggingLayer`, `TimeoutLayer`, `RetryLayer`, `CircuitBreakerLayer`, `ConcurrencyLimitLayer`, `MetricsLayer`, `ValidationLayer`
- Object pooling: `BufferPool`, `ByteSlicePool`, `StringSlicePool`
- Comprehensive test coverage (78%+)