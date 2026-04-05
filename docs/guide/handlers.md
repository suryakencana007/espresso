# Handlers

Handlers process requests and return responses. Espresso provides coffee-themed handler aliases for different use cases.

## Handler Types

<Mermaid source="graph LR
    A[Ristretto func Res] -->|0 params| B[Health checks Static responses]
    C[Solo func Req Res] -->|1 param| D[Simple handlers No context needed]
    E[Doppio func ctx Req Res] -->|2 params| F[Production use Full control]" />

| Handler | Signature | Use Case |
|---------|------------|----------|
| `Ristretto` | `func() Res` | Health checks, static responses |
| `Solo` | `func(*Req) (Res, error)` | Simple handlers, no context |
| `Doppio` | `func(ctx, *Req) (Res, error)` | Production handlers, full control |

## Ristretto (0 params)

Simplest handler for static responses:

```go
func healthCheck() espresso.Text {
    return espresso.Text{Body: "OK"}
}

func pong() espresso.Text {
    return espresso.Text{Body: "pong"}
}

app.Get("/health", espresso.Ristretto(healthCheck))
app.Get("/ping", espresso.Ristretto(pong))
```

**When to use:**
- Health checks
- Static responses
- No request processing needed

## Solo (1 param)

Handlers with request extraction:

```go
func createUser(req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    user := req.Data
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusCreated,
        Data: UserRes{ID: 1, Name: user.Name},
    }, nil
}

func search(req *extractor.Query[SearchReq]) (espresso.JSON[SearchRes], error) {
    return espresso.JSON[SearchRes]{
        Data: SearchRes{Results: []string{"item1"}, Query: req.Data.Query},
    }, nil
}

app.Post("/users", espresso.Solo(createUser))
app.Get("/search", espresso.Solo(search))
```

**When to use:**
- Simple CRUD operations
- Don't need context
- No middleware access needed

## Doppio (2 params)

Full control with context:

```go
func updateUser(ctx context.Context, req *espresso.JSON[UpdateUserReq]) (espresso.JSON[UserRes], error) {
    // Access context
    requestID := espresso.GetRequestID(ctx)
    logger, _ := espresso.GetLogger(ctx)
    
    // Access request data
    user := req.Data
    
    // Business logic
    logger.Info().Str("request_id", requestID).Msg("updating user")
    
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusOK,
        Data: UserRes{ID: user.ID, Name: user.Name},
    }, nil
}

app.Put("/users/{id}", espresso.Doppio(updateUser))
```

**When to use:**
- Production handlers
- Need context for tracing/auth
- Access to state/injections

## Handler Functions

For complete control without aliases:

```go
// HandlerCtxReqErr — func(ctx, *Req) (Res, error)
app.Post("/users", espresso.HandlerCtxReqErr(createUser))

// HandlerCtxReq — func(ctx, *Req) Res
app.Get("/users", espresso.HandlerCtxReq(listUsers))

// HandlerReqErr — func(*Req) (Res, error)
app.Post("/simple", espresso.HandlerReqErr(simpleHandler))

// HandlerReq — func(*Req) Res
app.Get("/simple", espresso.HandlerReq(simpleHandler))

// HandlerCtx — func(ctx) (Res, error)
app.Get("/context", espresso.HandlerCtx(contextHandler))

// HandlerNoReq — func() (Res, error)
app.Get("/static", espresso.HandlerNoReq(staticHandler))
```

## Service Interface

For reusable business logic:

```go
type UserService struct {
    db *sql.DB
}

func (s *UserService) Call(ctx context.Context, req *CreateUserReq) (espresso.JSON[UserRes], error) {
    user, err := s.db.CreateUser(req.Name, req.Email)
    if err != nil {
        return espresso.JSON[UserRes]{}, err
    }
    return espresso.JSON[UserRes]{Data: user}, nil
}

app.Post("/users", UserService{db: db})
```

## Response Types

### JSON Response

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

return espresso.JSON[User]{
    StatusCode: http.StatusCreated,
    Data: User{ID: 1, Name: "John"},
}
```

### Text Response

```go
return espresso.Text{Body: "OK"}
return espresso.Text{StatusCode: http.StatusNotFound, Body: "not found"}
```

### Status Only

```go
return espresso.Status(http.StatusNoContent)
return espresso.Status(http.StatusAccepted)
```

### Custom Response

Implement `IntoResponse`:

```go
type HTML struct {
    Body string
}

func (h HTML) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "text/html")
    _, err := w.Write([]byte(h.Body))
    return err
}
```

## WithLayers

Apply middleware to handlers:

```go
import servicemiddleware "github.com/suryakencana007/espresso/middleware/service"

layers := espresso.Layers(
    espresso.Timeout(5 * time.Second),
    espresso.Logging(logger, "api"),
)

app.Post("/users", espresso.WithLayers(createUser, layers...))
```

## Next Steps

- [Extractors](/guide/extractors) — Request extraction
- [Middleware](/guide/middleware/) — Middleware composition
- [State](/guide/state) — Dependency injection