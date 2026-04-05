package espresso

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

type testServiceReq struct {
	Name string `json:"name"`
}

type testServiceRes struct {
	Message string `json:"message"`
}

type mockService struct {
	callCount int
	err       error
}

func (s *mockService) Call(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
	s.callCount++
	if s.err != nil {
		return testServiceRes{}, s.err
	}
	return testServiceRes{Message: "hello " + req.Name}, nil
}

func TestBuildService(t *testing.T) {
	builder := BuildService[*testServiceReq, testServiceRes]()
	if builder == nil {
		t.Error("expected non-nil ServiceBuilder")
	}
}

func TestServiceBuilder_Layer(t *testing.T) {
	layerCallCount := 0
	layer := func(next Service[*testServiceReq, testServiceRes]) Service[*testServiceReq, testServiceRes] {
		return serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
			layerCallCount++
			return next.Call(ctx, req)
		})
	}

	builder := BuildService[*testServiceReq, testServiceRes]()
	result := builder.Layer(layer)

	if result == nil {
		t.Error("expected non-nil ServiceBuilder after Layer")
	}
}

func TestServiceBuilder_Service(t *testing.T) {
	svc := &mockService{}

	builder := BuildService[*testServiceReq, testServiceRes]()
	result := builder.Service(svc)

	if result == nil {
		t.Error("expected non-nil Service after Service()")
	}
}

func TestServiceBuilder_LayerOrder(t *testing.T) {
	order := []string{}

	layer1 := func(next Service[*testServiceReq, testServiceRes]) Service[*testServiceReq, testServiceRes] {
		return serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
			order = append(order, "layer1-before")
			res, err := next.Call(ctx, req)
			order = append(order, "layer1-after")
			return res, err
		})
	}

	layer2 := func(next Service[*testServiceReq, testServiceRes]) Service[*testServiceReq, testServiceRes] {
		return serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
			order = append(order, "layer2-before")
			res, err := next.Call(ctx, req)
			order = append(order, "layer2-after")
			return res, err
		})
	}

	svc := &mockService{}
	wrapped := BuildService[*testServiceReq, testServiceRes]().
		Layer(layer1).
		Layer(layer2).
		Service(svc)

	req := &testServiceReq{Name: "test"}
	_, err := wrapped.Call(context.Background(), req)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := []string{"layer1-before", "layer2-before", "layer2-after", "layer1-after"}
	if len(order) != len(expected) {
		t.Errorf("expected %d calls, got %d", len(expected), len(order))
	}
}

func TestServiceFunc(t *testing.T) {
	fn := func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
		return testServiceRes{Message: "hello " + req.Name}, nil
	}

	svc := serviceFunc[*testServiceReq, testServiceRes](fn)
	req := &testServiceReq{Name: "world"}

	res, err := svc.Call(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "hello world" {
		t.Errorf("expected message 'hello world', got '%s'", res.Message)
	}
}

func TestLayered(t *testing.T) {
	svc := &mockService{}
	layer := func(next Service[*testServiceReq, testServiceRes]) Service[*testServiceReq, testServiceRes] {
		return serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
			return next.Call(ctx, req)
		})
	}

	result := Layered(svc, layer)
	if result == nil {
		t.Error("expected non-nil Service from Layered")
	}
}

func TestLoggingLayer(t *testing.T) {
	svc := &mockService{}
	logger := zerolog.Nop()
	layer := buildLayer[*testServiceReq, testServiceRes](Logging(logger, "test-service"))

	wrapped := layer(svc)
	req := &testServiceReq{Name: "test"}

	_, err := wrapped.Call(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if svc.callCount != 1 {
		t.Errorf("expected service to be called once, got %d", svc.callCount)
	}
}

func TestTimeoutLayer_Success(t *testing.T) {
	svc := &mockService{}
	layer := buildLayer[*testServiceReq, testServiceRes](Timeout(1 * time.Second))

	wrapped := layer(svc)
	req := &testServiceReq{Name: "test"}

	res, err := wrapped.Call(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "hello test" {
		t.Errorf("expected message 'hello test', got '%s'", res.Message)
	}
}

func TestTimeoutLayer_Timeout(t *testing.T) {
	slowService := serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
		select {
		case <-time.After(2 * time.Second):
			return testServiceRes{Message: "done"}, nil
		case <-ctx.Done():
			return testServiceRes{}, ctx.Err()
		}
	})

	layer := buildLayer[*testServiceReq, testServiceRes](Timeout(100 * time.Millisecond))
	wrapped := layer(slowService)

	req := &testServiceReq{Name: "test"}
	_, err := wrapped.Call(context.Background(), req)

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRetryLayer_Success(t *testing.T) {
	svc := &mockService{}
	layer := buildLayer[*testServiceReq, testServiceRes](Retry(3, 10*time.Millisecond, servicemiddleware.BackoffFixed))

	wrapped := layer(svc)
	req := &testServiceReq{Name: "test"}

	res, err := wrapped.Call(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "hello test" {
		t.Errorf("expected message 'hello test', got '%s'", res.Message)
	}

	if svc.callCount != 1 {
		t.Errorf("expected service to be called once, got %d", svc.callCount)
	}
}

func TestRetryLayer_Retries(t *testing.T) {
	callCount := 0
	failingService := serviceFunc[*testServiceReq, testServiceRes](func(ctx context.Context, req *testServiceReq) (testServiceRes, error) {
		callCount++
		if callCount < 3 {
			return testServiceRes{}, errors.New("temporary error")
		}
		return testServiceRes{Message: "success"}, nil
	})

	layer := buildLayer[*testServiceReq, testServiceRes](Retry(5, 1*time.Millisecond, servicemiddleware.BackoffFixed))
	wrapped := layer(failingService)

	req := &testServiceReq{Name: "test"}
	res, err := wrapped.Call(context.Background(), req)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "success" {
		t.Errorf("expected success message, got '%s'", res.Message)
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestConcurrencyLimitLayer(t *testing.T) {
	svc := &mockService{}
	layer := buildLayer[*testServiceReq, testServiceRes](ConcurrencyLimit(1))

	wrapped := layer(svc)
	req := &testServiceReq{Name: "test"}

	res, err := wrapped.Call(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "hello test" {
		t.Errorf("expected message 'hello test', got '%s'", res.Message)
	}
}

func TestValidationLayer_Valid(t *testing.T) {
	svc := &mockService{}
	validator := servicemiddleware.ValidatorFunc[*testServiceReq](func(ctx context.Context, req *testServiceReq) error {
		return nil
	})

	layer := buildLayer[*testServiceReq, testServiceRes](Validation(validator))
	wrapped := layer(svc)

	req := &testServiceReq{Name: "test"}
	res, err := wrapped.Call(context.Background(), req)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if res.Message != "hello test" {
		t.Errorf("expected message 'hello test', got '%s'", res.Message)
	}
}

func TestValidationLayer_Invalid(t *testing.T) {
	svc := &mockService{}
	validator := servicemiddleware.ValidatorFunc[*testServiceReq](func(ctx context.Context, req *testServiceReq) error {
		return errors.New("validation failed")
	})

	layer := buildLayer[*testServiceReq, testServiceRes](Validation(validator))
	wrapped := layer(svc)

	req := &testServiceReq{Name: "test"}
	_, err := wrapped.Call(context.Background(), req)

	if err == nil {
		t.Error("expected validation error")
	}
}

func TestErrValidation(t *testing.T) {
	err := servicemiddleware.ErrValidation{Err: errors.New("test error")}
	if err.Error() != "validation error: test error" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	unwrapped := err.Unwrap()
	if unwrapped.Error() != "test error" {
		t.Errorf("unexpected unwrapped error: %s", unwrapped.Error())
	}
}
