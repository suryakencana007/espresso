# Error Handling

## Overview

Espresso provides structured error responses using the `*espresso.Error` type. Handler errors are automatically serialized as JSON with consistent format, error codes, and request ID integration.

## Returning Structured Errors

### Using Constructors

```go
func getUser(ctx context.Context, req *espresso.JSON[GetUserReq]) (espresso.JSON[User], error) {
    user, err := db.FindUser(req.Data.ID)
    if err != nil {
        return espresso.JSON[User]{}, espresso.ErrNotFound("user not found")
    }
    return espresso.JSON[User]{Data: user}, nil
}
```

Available constructors:

| Constructor | Status | Code |
|-------------|--------|------|
| `ErrBadRequest(msg)` | 400 | `BAD_REQUEST` |
| `ErrUnauthorized(msg)` | 401 | `UNAUTHORIZED` |
| `ErrForbidden(msg)` | 403 | `FORBIDDEN` |
| `ErrNotFound(msg)` | 404 | `NOT_FOUND` |
| `ErrConflict(msg)` | 409 | `CONFLICT` |
| `ErrPreconditionFailed(msg)` | 412 | `PRECONDITION_FAILED` |
| `ErrUnprocessableEntity(msg)` | 422 | `UNPROCESSABLE_ENTITY` |
| `ErrTooManyRequests(msg)` | 429 | `TOO_MANY_REQUESTS` |
| `ErrInternal(msg)` | 500 | `INTERNAL` |
| `ErrServiceUnavailable(msg)` | 503 | `SERVICE_UNAVAILABLE` |

### Custom Error Codes

```go
return espresso.NewError(http.StatusNotFound, "project not found").
    WithCode("PROJECT_NOT_FOUND")
```

### Adding Details

```go
return espresso.ErrBadRequest("validation failed").
    WithDetail("field", "email").
    WithDetail("error", "invalid format")

// Or multiple at once:
return espresso.ErrBadRequest("validation failed").
    WithDetails(map[string]any{
        "errors": []espresso.ValidationError{
            {Field: "email", Message: "required"},
            {Field: "name", Message: "too short"},
        },
    })
```

### Wrapping Internal Errors

```go
return espresso.ErrInternal("database error").
    Wrap(dbErr)
```

The wrapped error is logged but **never exposed** to the client. Only the message and code are visible in the response.

### Request ID Integration

When `RequestIDMiddleware` is used, error responses automatically include the request ID from the context:

```go
router := espresso.Portafilter().
    Use(httpmiddleware.RequestIDMiddleware())
```

Response:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "user not found",
    "request_id": "req-abc123"
  }
}
```

## Error Response Format

All error responses follow this JSON structure:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "user not found",
    "details": { "userId": "abc123" },
    "request_id": "req-abc123"
  }
}
```

| Field | Description |
|-------|-------------|
| `code` | Machine-readable error code (e.g., `NOT_FOUND`) |
| `message` | Human-readable error message |
| `details` | Optional additional context (omitted if empty) |
| `request_id` | Request ID from context (omitted if not available) |

## Validation Errors

```go
errors := []espresso.ValidationError{
    {Field: "email", Message: "required"},
    {Field: "name", Message: "must be at least 3 characters"},
}
return espresso.ValidationErrors(errors)
```

Response:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "One or more fields failed validation",
    "details": {
      "errors": [
        {"field": "email", "message": "required"},
        {"field": "name", "message": "must be at least 3 characters"}
      ]
    }
  }
}
```

## Panic Recovery

With `RecoverMiddleware`, panics are caught and converted to structured JSON:

```go
router.Use(httpmiddleware.RecoverMiddleware())
```

Response on panic:

```json
{
  "error": {
    "code": "PANIC",
    "message": "internal server error",
    "request_id": "req-abc123"
  }
}
```

## Plain Error Returns

Handlers can still return plain `error` values for backward compatibility:

```go
func handler(ctx context.Context, req *espresso.JSON[Req]) (espresso.JSON[Res], error) {
    return espresso.JSON[Res]{}, fmt.Errorf("something went wrong")
}
```

This produces a 500 response:

```json
{
  "error": {
    "code": "INTERNAL",
    "message": "internal server error"
  }
}
```

## Backward Compatibility

The following old-style constructors still work:

```go
espresso.BadRequest("invalid request")
espresso.NotFound("resource not found")
espresso.InternalError("something failed")
```

These now return `*espresso.Error` and produce the same JSON format. `ErrorResponse` is a type alias for `Error`.

## Error Code Naming Convention

Use `UPPER_SNAKE_CASE` for error codes. Group by domain:

- `USER_NOT_FOUND`, `USER_ALREADY_EXISTS`
- `PROJECT_NOT_FOUND`, `PROJECT_ARCHIVED`
- `VALIDATION_ERROR` (for field validation)

## Best Practices

1. **Never expose internal errors to clients.** Use `.Wrap()` for logging, not for client responses.
2. **Use specific error codes** so clients can handle errors programmatically.
3. **Use `.WithDetail()` sparingly** — not all errors need extra details.
4. **Always use RequestIDMiddleware** so errors can be traced in logs.
5. **Return `*espresso.Error` from handlers** for proper status codes and JSON format.

## See Also

- [WebSocket](websocket.md) - WebSocket handlers
- [Streaming (SSE)](streaming.md) - SSE streaming