// Package bench compares Espresso against popular Go web frameworks on three
// equivalent scenarios: static text response, JSON round-trip, and path
// parameter extraction. Measurements use httptest.ResponseRecorder (or the
// framework-native equivalent for Fiber, which sits on fasthttp rather than
// net/http) and report ns/op + B/op + allocs/op.
//
// Caveat: Fiber's results include fasthttp's test harness overhead, which is
// not strictly comparable to net/http dispatch for the other three. Treat
// Fiber's numbers as an order-of-magnitude hint, not a head-to-head.
package bench

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/labstack/echo/v4"
	"github.com/suryakencana007/espresso"
	"github.com/suryakencana007/espresso/extractor"
)

type echoReq struct {
	Name string `json:"name"`
}

type echoRes struct {
	Greeting string `json:"greeting"`
}

type userIDPath struct {
	ID string `path:"id"`
}

type userRes struct {
	ID string `json:"id"`
}

// ============================================================================
// Scenario 1: static text GET
// ============================================================================

func BenchmarkStaticText_Espresso(b *testing.B) {
	router := espresso.Portafilter().Get("/ping", espresso.Ristretto(func() espresso.Text {
		return espresso.Text{Body: "pong"}
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	runNetHTTP(b, router, req)
}

func BenchmarkStaticText_Gin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	runNetHTTP(b, r, req)
}

func BenchmarkStaticText_Echo(b *testing.B) {
	e := echo.New()
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	runNetHTTP(b, e, req)
}

func BenchmarkStaticText_Fiber(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	runFiber(b, app, req)
}

// ============================================================================
// Scenario 2: JSON round-trip POST
// ============================================================================

const jsonBody = `{"name":"world"}`

func BenchmarkJSON_Espresso(b *testing.B) {
	router := espresso.Portafilter().Post("/echo",
		espresso.Doppio(func(ctx context.Context, req *espresso.JSON[echoReq]) (espresso.JSON[echoRes], error) {
			return espresso.JSON[echoRes]{Data: echoRes{Greeting: "hello " + req.Data.Name}}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	runNetHTTPBody(b, router, req, jsonBody)
}

func BenchmarkJSON_Gin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/echo", func(c *gin.Context) {
		var req echoReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusOK, echoRes{Greeting: "hello " + req.Name})
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	runNetHTTPBody(b, r, req, jsonBody)
}

func BenchmarkJSON_Echo(b *testing.B) {
	e := echo.New()
	e.POST("/echo", func(c echo.Context) error {
		var req echoReq
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, echoRes{Greeting: "hello " + req.Name})
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	runNetHTTPBody(b, e, req, jsonBody)
}

func BenchmarkJSON_Fiber(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/echo", func(c *fiber.Ctx) error {
		var req echoReq
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		return c.JSON(echoRes{Greeting: "hello " + req.Name})
	})

	runFiberBody(b, app, jsonBody)
}

// ============================================================================
// Scenario 3: path parameter GET
// ============================================================================

func BenchmarkPathParam_Espresso(b *testing.B) {
	router := espresso.Portafilter().Get("/users/{id}",
		espresso.Doppio(func(ctx context.Context, req *extractor.Path[userIDPath]) (espresso.JSON[userRes], error) {
			return espresso.JSON[userRes]{Data: userRes{ID: req.Data.ID}}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	runNetHTTP(b, router, req)
}

func BenchmarkPathParam_Gin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, userRes{ID: c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	runNetHTTP(b, r, req)
}

func BenchmarkPathParam_Echo(b *testing.B) {
	e := echo.New()
	e.GET("/users/:id", func(c echo.Context) error {
		return c.JSON(http.StatusOK, userRes{ID: c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	runNetHTTP(b, e, req)
}

func BenchmarkPathParam_Fiber(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return c.JSON(userRes{ID: c.Params("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	runFiber(b, app, req)
}

// ============================================================================
// Test runners
// ============================================================================

func runNetHTTP(b *testing.B, h http.Handler, req *http.Request) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
	}
}

// runNetHTTPBody rewinds the body reader each iteration, which isolates the
// handler cost from body-allocation overhead we control separately.
func runNetHTTPBody(b *testing.B, h http.Handler, req *http.Request, body string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = io.NopCloser(strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
	}
}

func runFiber(b *testing.B, app *fiber.App, req *http.Request) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, -1)
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
}

// runFiberBody rebuilds the request per iteration because Fiber's Test harness
// serializes to wire format and requires an accurate Content-Length; swapping
// req.Body in-place leaves the pre-set length stale.
func runFiberBody(b *testing.B, app *fiber.App, body string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
}
