package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestAppState struct {
	DB     string
	Config string
}

func TestGetState_Success(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	ctx := context.WithValue(context.Background(), stateKey{}, appState)

	state, ok := GetState[TestAppState](ctx)
	if !ok {
		t.Error("expected state to be found")
	}

	if state.DB != "testdb" {
		t.Errorf("expected DB 'testdb', got '%s'", state.DB)
	}

	if state.Config != "testconfig" {
		t.Errorf("expected Config 'testconfig', got '%s'", state.Config)
	}
}

func TestGetState_NotFound(t *testing.T) {
	ctx := context.Background()

	state, ok := GetState[TestAppState](ctx)
	if ok {
		t.Error("expected state to not be found")
	}

	if state.DB != "" {
		t.Errorf("expected zero value for DB, got '%s'", state.DB)
	}
}

func TestMustGetState_Success(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	ctx := context.WithValue(context.Background(), stateKey{}, appState)

	state := MustGetState[TestAppState](ctx)
	if state.DB != "testdb" {
		t.Errorf("expected DB 'testdb', got '%s'", state.DB)
	}
}

func TestMustGetState_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when state not found")
		}
	}()

	ctx := context.Background()
	MustGetState[TestAppState](ctx)
}

func TestWithStateMiddleware(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	middleware := WithStateMiddleware(appState)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state, ok := GetState[TestAppState](r.Context())
		if !ok {
			t.Error("expected state to be found in handler")
		}
		if state.DB != "testdb" {
			t.Errorf("expected DB 'testdb', got '%s'", state.DB)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestState_Extract_Success(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), stateKey{}, appState)
	req = req.WithContext(ctx)

	state := &State[TestAppState]{}
	err := state.Extract(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if state.Data.DB != "testdb" {
		t.Errorf("expected DB 'testdb', got '%s'", state.Data.DB)
	}
}

func TestState_Extract_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	state := &State[TestAppState]{}
	err := state.Extract(req)
	if err == nil {
		t.Error("expected error when state not found")
	}

	var stateErr *StateNotFoundError
	if err != nil {
		if !asStateError(err, &stateErr) {
			t.Errorf("expected StateNotFoundError, got %T", err)
		}
	}
}

func TestState_Reset(t *testing.T) {
	state := &State[TestAppState]{
		Data: TestAppState{DB: "testdb", Config: "testconfig"},
	}

	state.Reset()

	if state.Data.DB != "" {
		t.Errorf("expected empty DB after reset, got '%s'", state.Data.DB)
	}
}

func TestFromState_Success(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	ctx := context.WithValue(context.Background(), stateKey{}, appState)

	db, ok := FromState[TestAppState, string](ctx, func(s TestAppState) string {
		return s.DB
	})

	if !ok {
		t.Error("expected state to be found")
	}

	if db != "testdb" {
		t.Errorf("expected DB 'testdb', got '%s'", db)
	}
}

func TestFromState_NotFound(t *testing.T) {
	ctx := context.Background()

	db, ok := FromState[TestAppState, string](ctx, func(s TestAppState) string {
		return s.DB
	})

	if ok {
		t.Error("expected state to not be found")
	}

	if db != "" {
		t.Errorf("expected empty string, got '%s'", db)
	}
}

func TestMustFromState_Success(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	ctx := context.WithValue(context.Background(), stateKey{}, appState)

	db := FromMustState[TestAppState, string](ctx, func(s TestAppState) string {
		return s.DB
	})

	if db != "testdb" {
		t.Errorf("expected DB 'testdb', got '%s'", db)
	}
}

func TestMustFromState_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when state not found")
		}
	}()

	ctx := context.Background()
	FromMustState[TestAppState, string](ctx, func(s TestAppState) string {
		return s.DB
	})
}

func TestRouter_WithState(t *testing.T) {
	appState := TestAppState{
		DB:     "testdb",
		Config: "testconfig",
	}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		state, ok := GetState[TestAppState](r.Context())
		if !ok {
			t.Error("expected state to be found in handler")
		}
		if state.DB != "testdb" {
			t.Errorf("expected DB 'testdb', got '%s'", state.DB)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := WithStateMiddleware(appState)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

type StateTestReq struct {
	Msg string `json:"msg"`
}

func (r *StateTestReq) Extract(req *http.Request) error {
	r.Msg = "test"
	return nil
}

func TestIntegration_StateWithHandler(t *testing.T) {
	appState := TestAppState{
		DB:     "production_db",
		Config: "production_config",
	}

	router := Portafilter().
		WithState(appState).
		Get("/test", Handler(func(ctx context.Context) (Text, error) {
			state := MustGetState[TestAppState](ctx)
			return Text{Body: state.DB + "_" + state.Config}, nil
		}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	expected := "production_db_production_config"
	if body != expected {
		t.Errorf("expected body '%s', got '%s'", expected, body)
	}
}

func asStateError(err error, target **StateNotFoundError) bool {
	if err == nil {
		return false
	}
	*target, _ = err.(*StateNotFoundError)
	return *target != nil
}
