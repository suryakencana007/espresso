---
title: Extractor API Reference
description: Request extractor types and functions
---

# Extractor API Reference

Package `extractor` provides types for extracting data from HTTP requests.

```go
import "github.com/suryakencana007/espresso/extractor"
```

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
}

func handler(ctx context.Context, req *extractor.Cookie[SessionCookies]) (Response, error) {
    return Response{SessionID: req.Data.SessionID}, nil
}
```

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