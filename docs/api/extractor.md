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
type XML[T any] = XMLExtractor[T]
type RawBody = RawBodyExtractor
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