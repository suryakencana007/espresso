# Extractors

Extractors extract and parse data from HTTP requests into typed Go structs. They follow the Axum pattern where request types implement `FromRequest` interface.

## Built-in Extractors

### JSON

Extract JSON request body into a typed struct:

```go
type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (Response, error) {
    // req.Data contains the parsed JSON
    user := User{
        Name:  req.Data.Name,
        Email: req.Data.Email,
    }
    return espresso.JSON[User]{Data: user}, nil
}

router.Post("/users", espresso.Doppio(createUser))
```

### Query

Extract URL query parameters:

```go
type Pagination struct {
    Page    int    `query:"page"`
    PerPage int    `query:"per_page"`
    Sort    string `query:"sort"`
}

func listUsers(ctx context.Context, req *espresso.Query[Pagination]) (Response, error) {
    page := req.Data.Page
    if page == 0 {
        page = 1 // Default
    }
    // ...
}

router.Get("/users", espresso.Doppio(listUsers))
// Request: GET /users?page=2&per_page=50&sort=name
```

#### Required Query Parameters

```go
type SearchReq struct {
    Q string `query:"q,required"` // Returns error if missing
}
```

### Path

Extract path parameters from URLs (Go 1.22+):

```go
type UserPath struct {
    ID int64 `path:"id,required"`
}

func getUser(ctx context.Context, req *espresso.Path[UserPath]) (Response, error) {
    user := findUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}

router.Get("/users/{id}", espresso.Doppio(getUser))
```

### Form

Extract form data (`application/x-www-form-urlencoded`):

```go
type LoginForm struct {
    Email    string `form:"email,required"`
    Password string `form:"password,required"`
}

func login(ctx context.Context, req *espresso.Form[LoginForm]) (Response, error) {
    // Authenticate user
    return espresso.JSON[Token]{Data: token}, nil
}

router.Post("/login", espresso.Doppio(login))
```

### Header

Extract HTTP headers:

```go
type AuthHeaders struct {
    Authorization string `header:"Authorization,required"`
    RequestID     string `header:"X-Request-ID"`
}

func handler(ctx context.Context, req *espresso.Header[AuthHeaders]) (Response, error) {
    token := req.Data.Authorization
    // ...
}

router.Get("/protected", espresso.Doppio(handler))
```

### XML

Extract XML request body:

```go
type XMLRequest struct {
    XMLName xml.Name `xml:"request"`
    Value   string   `xml:"value"`
}

func handler(ctx context.Context, req *espresso.XML[XMLRequest]) (Response, error) {
    // req.Data contains parsed XML
    return espresso.XML[XMLResponse]{Data: response}, nil
}

router.Post("/xml", espresso.Doppio(handler))
```

### RawBody

Get raw request body as bytes:

```go
func handler(ctx context.Context, req *espresso.RawBody) (Response, error) {
    data := req.Data // []byte
    // Process raw data
    return espresso.Text{Body: "processed"}, nil
}

router.Post("/raw", espresso.Solo(handler))
```

### Multipart

Handle multipart/form-data with file uploads:

```go
type UploadForm struct {
    Title       string          `form:"title"`
    Description string          `form:"description"`
    Filename    string          `file:"document"`
}

func handler(ctx context.Context, req *extractor.Multipart[UploadForm]) (Response, error) {
    title := req.Data.Title
    filename := req.Data.Filename
    // Process form with file
    return espresso.JSON[Response]{Data: Response{Status: "uploaded"}}, nil
}

router.Post("/upload", espresso.Doppio(handler))
```

### File

Handle single file uploads:

```go
func handler(ctx context.Context, req *extractor.File) (Response, error) {
    filename := req.File.Filename
    size := req.File.Size
    // File metadata only, not content
    return espresso.JSON[Response]{Data: Response{
        Filename: filename,
        Size:     size,
    }}, nil
}

router.Post("/upload", espresso.Doppio(handler))
```

For file content, access via `r.FormFile("file")` in your handler.

### Files

Handle multiple file uploads:

```go
func handler(ctx context.Context, req *extractor.Files) (Response, error) {
    count := len(req.Files)
    filenames := make([]string, 0, count)
    for _, f := range req.Files {
        filenames = append(filenames, f.Filename)
    }
    return espresso.JSON[Response]{Data: Response{
        Count:     count,
        Filenames: filenames,
    }}, nil
}

router.Post("/upload/multiple", espresso.Doppio(handler))
```

## Combining Extractors

Use multiple extractors in a single handler:

```go
type UserRequest struct {
    // Path parameter
    ID int64 `path:"id,required"`
}

type UserQuery struct {
    // Query parameters
    Fields string `query:"fields"`
}

func getUser(
    ctx context.Context,
    path *espresso.Path[UserRequest],
    query *espresso.Query[UserQuery],
) (Response, error) {
    user := findUser(path.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}

// Currently requires manual extraction until multi-extractor support
func getUserHandler(ctx context.Context, req *espresso.JSON[struct{}]) (Response, error) {
    // Extract path params manually
    r := espresso.RequestFromContext(ctx)
    id := r.PathValue("id")
    
    // Extract query params
    var query UserQuery
    if err := (&espresso.QueryExtractor[UserQuery]{Data: query}).Extract(r); err != nil {
        return nil, err
    }
    
    // Process...
}
```

## Error Handling

Extractors return typed errors for validation:

```go
func handler(ctx context.Context, req *espresso.JSON[CreateUserReq]) (Response, error) {
    // The extractor already validated and returned errors
    // If we reach here, extraction succeeded
    
    // Manual validation
    if req.Data.Name == "" {
        return nil, &extractor.FieldError{
            Field:   "name",
            Message: "name is required",
        }
    }
    
    return espresso.JSON[User]{Data: user}, nil
}
```

### Field Errors

```go
type FieldError struct {
    Field   string
    Message string
    Value   any
}

type FieldErrors []FieldError // Multiple errors

func (fe FieldErrors) ToValidationErrors() []ValidationError {
    // Convert to JSON-friendly format
}
```

## Custom Extractors

Implement `FromRequest` interface:

```go
type FromRequest interface {
    Extract(r *http.Request) error
}

// Custom extractor for authenticated user
type AuthenticatedUser struct {
    ID    int64
    Email string
}

func (u *AuthenticatedUser) Extract(r *http.Request) error {
    token := r.Header.Get("Authorization")
    if token == "" {
        return errors.New("missing authorization token")
    }
    
    user, err := validateToken(token)
    if err != nil {
        return err
    }
    
    u.ID = user.ID
    u.Email = user.Email
    return nil
}
```

Use custom extractor:

```go
func handler(ctx context.Context, user AuthenticatedUser) (Response, error) {
    // user already authenticated
    return espresso.JSON[Profile]{Data: user.Profile}, nil
}

router.Get("/profile", espresso.Solo(handler))
```

## Object Pooling

Extractors implement `Reset()` for object pooling:

```go
func (j *JSON[T]) Reset() {
    j.StatusCode = 0
    var zero T
    j.Data = zero
}
```

This allows pooling extractors for high-performance scenarios:

```go
var jsonPool = sync.Pool{
    New: func() any { return &espresso.JSON[MyRequest]{} },
}

req := jsonPool.Get().(*espresso.JSON[MyRequest])
defer func() {
    req.Reset()
    jsonPool.Put(req)
}()
```

## Performance Tips

1. **Use pointer types** for request extractors: `*JSON[T]` instead of `JSON[T]`
2. **Mark required fields** to fail fast
3. **Use specific types** instead of `any`
4. **Reuse extractor instances** in hot paths
5. **Consider raw body** for large payloads