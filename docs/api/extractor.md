---
title: Extractor API Reference
description: Request extractor types and functions
---

# Extractor API Reference

Package `extractor` provides types for extracting data from HTTP requests.

```go
import "github.com/suryakencana007/espresso/extractor"
```

::: tip Auto-validation since v2.0
Every extractor below calls `espresso.RunDefaultValidator(&data)` at the
end of `Extract`. When `espresso.SetDefaultValidator` is unset (the
default), this is a single atomic load — no behavioral change. When set,
malformed-by-tag inputs are rejected with a structured 400 before the
handler runs. See [Auto-Validation Hook](espresso.md#auto-validation-hook).
:::

## Core Interface

### FromRequest

All extractors implement this interface:

```go
type FromRequest interface {
    Extract(r *http.Request) error
}
```

### Resettable

Extractors implement `Reset()` for object pooling:

```go
type Resettable interface {
    Reset()
}
```

## Extractor Types

### JSON

Extract JSON request body:

```go
type JSON[T any] struct {
    Data T
}

func (j *JSON[T]) Extract(r *http.Request) error
func (j *JSON[T]) Reset()
func (j *JSON[T]) WriteResponse(w http.ResponseWriter) error
```

### Query

Extract URL query parameters:

```go
type Query[T any] struct {
    Data T
}
```

Usage:
```go
type Params struct {
    Page    int    `query:"page"`
   PerPage  int    `query:"per_page"`
    Search string `query:"search,required"`
}
```

### Path

Extract path parameters (Go 1.22+):

```go
type Path[T any] struct {
    Data T
}
```

Usage:
```go
type UserPath struct {
    ID int64 `path:"id,required"`
}
```

### Form

Extract form data:

```go
type Form[T any] struct {
    Data T
}
```

Usage:
```go
type Login struct {
    Email    string `form:"email,required"`
    Password string `form:"password,required"`
}
```

### Header

Extract HTTP headers:

```go
type Header[T any] struct {
    Data T
}
```

Usage:
```go
type AuthHeader struct {
    Authorization string `header:"Authorization,required"`
    RequestID string `header:"X-Request-ID"`
}
```

### Cookie

Extract HTTP cookies:

```go
type Cookie[T any] struct {
    Data T
}
```

Usage:
```go
type SessionCookies struct {
    SessionID string `cookie:"session_id,required"`
    UserID    string `cookie:"user_id"`
    Theme     string `cookie:"theme"`
}

func handler(ctx context.Context, req *extractor.Cookie[SessionCookies]) (espresso.JSON[User], error) {
    sessionID := req.Data.SessionID
    userID := req.Data.UserID
    return espresso.JSON[User]{Data: User{ID: userID}}, nil
}

router.Get("/profile", espresso.Doppio(handler))
```

Supported types: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`.

### Multipart

Extract multipart/form-data with file uploads:

```go
type Multipart[T any] struct {
    Data T
}
```

Usage:
```go
type UploadForm struct {
    Title    string `form:"title"`
    Filename string `file:"document"`
}

func handler(ctx context.Context, req *extractor.Multipart[UploadForm]) (Response, error) {
    return Response{Title: req.Data.Title}, nil
}
```

### FileInfo

File metadata from uploads:

```go
type FileInfo struct {
    Filename string
    Size     int64
    Header   textproto.MIMEHeader
}
```

### File

Extract single file upload:

```go
type File struct {
    File FileInfo
}
```

Usage:
```go
func handler(ctx context.Context, req *extractor.File) (Response, error) {
    return Response{Filename: req.File.Filename}, nil
}
```

### Files

Extract multiple file uploads:

```go
type Files struct {
    Files []FileInfo
}
```

Usage:
```go
func handler(ctx context.Context, req *extractor.Files) (Response, error) {
    return Response{Count: len(req.Files)}, nil
}
```

### XML

Extract XML request body:

```go
type XML[T any] struct {
    Data T
}
```

### RawBody

Extract raw request body:

```go
type RawBody struct {
    Data []byte
}
```

### RawBodyWithHeaders

Extract the raw request body alongside structured headers in a single read pass.
Useful for webhook receivers that need to verify HMAC against the unparsed
payload while reading provider-specific signature headers.

```go
type RawBodyWithHeaders[H any] struct {
    Body    []byte
    Headers H
}
```

The `H` type parameter uses the same `header:"Name,required"` struct-tag
convention as `Header[T]`. The framework never decodes the body, so HMAC
computations operate on bytes the sender produced.

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "strings"

    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
)

type GitHubHeaders struct {
    Signature string `header:"X-Hub-Signature-256,required"`
    Event     string `header:"X-GitHub-Event,required"`
}

func githubWebhook(ctx context.Context, req *extractor.RawBodyWithHeaders[GitHubHeaders]) (espresso.Status, error) {
    if !verifyHMAC(req.Body, req.Headers.Signature, secret) {
        return 0, espresso.ErrUnauthorized("invalid signature")
    }
    // dispatch on req.Headers.Event...
    return espresso.Status(http.StatusNoContent), nil
}

func verifyHMAC(body []byte, signature, secret string) bool {
    expected := strings.TrimPrefix(signature, "sha256=")
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    got := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(got))
}
```

## Error Types

### FieldError

Single field validation error:

```go
type FieldError struct {
    Field   string
    Message string
    Value   any
    Path    string
}
```

### FieldErrors

Multiple field errors:

```go
type FieldErrors []FieldError

func (fe FieldErrors) ToValidationErrors() []ValidationError
```

### TypeConversionError

Failed type conversion:

```go
type TypeConversionError struct {
    Field    string
    Expected string
    Actual   string
    Value    any
}
```

### UnsupportedTypeError

Unsupported field type:

```go
type UnsupportedTypeError struct {
    Field    string
    Expected string
    Actual   string
}
```

## Type Aliases

```go
type Query[T any] = QueryExtractor[T]
type Form[T any] = FormExtractor[T]
type Path[T any] = PathExtractor[T]
type Header[T any] = HeaderExtractor[T]
type Cookie[T any] = CookieExtractor[T]
type XML[T any] = XMLExtractor[T]
type Multipart[T any] = MultipartExtractor[T]
type RawBody = RawBodyExtractor
type File = FileExtractor
type Files = FilesExtractor
```

## Helper Functions

### PathParams

```go
func SetPathParams(r *http.Request, params PathParams) *http.Request
func GetPathParams(r *http.Request) PathParams
```

### RequiredFieldError

```go
func RequiredFieldError(field string, path ...string) FieldError
```

### InvalidTypeError

```go
func InvalidTypeError(field string, expected, actual string, value any, path ...string) FieldError
```

## See Also

- [Extractors Guide](/guide/extractors) - Extractor usage
- [Handlers Guide](/guide/handlers) - Handler patterns