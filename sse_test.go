package espresso

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSSE_BasicStream(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Name: "message", Data: "hello"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	cc := resp.Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		t.Fatal("expected to read a line")
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "id: ") {
		t.Errorf("expected id line, got %q", line)
	}

	if !scanner.Scan() {
		t.Fatal("expected event line")
	}
	if scanner.Text() != "event: message" {
		t.Errorf("expected event: message, got %q", scanner.Text())
	}

	if !scanner.Scan() {
		t.Fatal("expected data line")
	}
	if scanner.Text() != "data: hello" {
		t.Errorf("expected data: hello, got %q", scanner.Text())
	}
}

func TestSSE_MultipleEvents(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		_ = stream.Send(Event{Name: "count", Data: "1"})
		_ = stream.Send(Event{Name: "count", Data: "2"})
		_ = stream.Send(Event{Name: "count", Data: "3"})
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	events := parseSSEEvents(t, resp)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Name != "count" || events[0].Data != "1" {
		t.Errorf("event 0: expected name=count data=1, got name=%s data=%s", events[0].Name, events[0].Data)
	}
	if events[2].Name != "count" || events[2].Data != "3" {
		t.Errorf("event 2: expected name=count data=3, got name=%s data=%s", events[2].Name, events[2].Data)
	}
}

func TestSSE_JSONEvent(t *testing.T) {
	type msg struct {
		Text string `json:"text"`
	}

	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.SendJSON("update", msg{Text: "hello"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	var foundData bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, `data: {"text":"hello"}`) {
			foundData = true
			break
		}
	}
	if !foundData {
		t.Error("expected JSON data in SSE stream")
	}
}

func TestSSE_MultilineData(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Name: "multi", Data: "line1\nline2\nline3"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	s := string(body[:n])

	if !strings.Contains(s, "data: line1\n") {
		t.Error("expected data: line1 in output")
	}
	if !strings.Contains(s, "data: line2\n") {
		t.Error("expected data: line2 in output")
	}
	if !strings.Contains(s, "data: line3\n") {
		t.Error("expected data: line3 in output")
	}
}

func TestSSE_ClientDisconnect(t *testing.T) {
	var handlerDone atomic.Bool

	handler := func(ctx context.Context, stream *SSEStream) error {
		defer handlerDone.Store(true)
		_ = stream.Send(Event{Name: "ping", Data: "1"})
		<-ctx.Done()
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	// Read the initial event
	buf := make([]byte, 1024)
	_, _ = resp.Body.Read(buf)

	// Close the connection
	_ = resp.Body.Close()

	time.Sleep(500 * time.Millisecond)

	if !handlerDone.Load() {
		t.Error("handler did not finish after client disconnect")
	}
}

func TestSSE_LastEventID(t *testing.T) {
	var receivedID atomic.Value

	handler := func(ctx context.Context, stream *SSEStream) error {
		receivedID.Store(stream.LastEventID())
		_ = stream.Send(Event{Name: "msg", Data: "hello"})
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/stream", nil)
	req.Header.Set("Last-Event-ID", "42")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	buf := make([]byte, 4096)
	_, _ = resp.Body.Read(buf)

	if receivedID.Load() != "42" {
		t.Errorf("expected Last-Event-ID '42', got %v", receivedID.Load())
	}
}

func TestSSE_KeepAlive(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		<-ctx.Done()
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler, WithKeepAlive(50*time.Millisecond)))
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	gotKeepalive := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ": keepalive") {
			gotKeepalive = true
			break
		}
	}

	if !gotKeepalive {
		t.Error("expected keepalive comment in SSE stream")
	}
}

func TestSSE_StateInjection(t *testing.T) {
	type appState struct {
		Message string
	}

	state := appState{Message: "from state"}

	handler := func(ctx context.Context, stream *SSEStream) error {
		s := MustGetState[appState](ctx)
		return stream.Send(Event{Name: "msg", Data: s.Message})
	}

	router := Portafilter().WithState(state).Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	events := parseSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Data != "from state" {
		t.Errorf("expected data 'from state', got %q", events[0].Data)
	}
}

type stateStreamReq struct {
	name string
}

func (s *stateStreamReq) Extract(r *http.Request) error {
	s.name = r.URL.Query().Get("name")
	return nil
}

func TestSSE_Stream_StateInjection(t *testing.T) {
	type appState struct {
		Prefix string
	}

	state := appState{Prefix: "hello"}

	handler := func(ctx context.Context, req *stateStreamReq, stream *SSEStream) error {
		s := MustGetState[appState](ctx)
		return stream.Send(Event{Name: "msg", Data: s.Prefix + " " + req.name})
	}

	router := Portafilter().WithState(state).Get("/stream", Stream(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream?name=world")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	events := parseSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Data != "hello world" {
		t.Errorf("expected data 'hello world', got %q", events[0].Data)
	}
}

func TestSSE_ConcurrentWrites(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				_ = stream.Send(Event{Name: "msg", Data: "ABCDEFGHIJ"[n : n+1]})
			}(i)
		}
		wg.Wait()
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	events := parseSSEEvents(t, resp)
	if len(events) != 10 {
		t.Errorf("expected 10 events, got %d", len(events))
	}
}

func TestSSE_AutoEventID(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		_ = stream.Send(Event{Name: "first", Data: "1"})
		_ = stream.Send(Event{Name: "second", Data: "2"})
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	ids := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "id: ") {
			ids = append(ids, strings.TrimPrefix(line, "id: "))
		}
	}

	if len(ids) < 2 {
		t.Fatalf("expected at least 2 event IDs, got %d", len(ids))
	}

	if ids[0] != "1" {
		t.Errorf("expected first ID '1', got %q", ids[0])
	}
	if ids[1] != "2" {
		t.Errorf("expected second ID '2', got %q", ids[1])
	}
}

func TestSSE_Comment(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		_ = stream.Comment("this is a comment")
		return stream.Send(Event{Name: "msg", Data: "data"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	s := string(body[:n])

	if !strings.Contains(s, ": this is a comment\n") {
		t.Error("expected comment line in SSE output")
	}
}

func TestSSE_SetRetry(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		_ = stream.SetRetry(5 * time.Second)
		return stream.Send(Event{Name: "msg", Data: "hello"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	foundRetry := false
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "retry: 5000") {
			foundRetry = true
			break
		}
	}

	if !foundRetry {
		t.Error("expected retry directive in SSE stream")
	}
}

func TestSSE_HeadersSet(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Data: "test"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	tests := []struct {
		header string
		want   string
	}{
		{"Content-Type", "text/event-stream"},
		{"Cache-Control", "no-cache"},
		{"Connection", "keep-alive"},
		{"X-Accel-Buffering", "no"},
	}

	for _, tt := range tests {
		got := resp.Header.Get(tt.header)
		if got != tt.want {
			t.Errorf("header %s: expected %q, got %q", tt.header, tt.want, got)
		}
	}
}

func TestSSE_SendData(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.SendData("plain data")
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	events := parseSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Data != "plain data" {
		t.Errorf("expected data 'plain data', got %q", events[0].Data)
	}
	if events[0].Name != "" {
		t.Errorf("expected no event name, got %q", events[0].Name)
	}
}

func TestSSE_WithRetryHint(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.SendData("hello")
	}

	router := Portafilter().Get("/stream", StreamSimple(handler, WithRetryHint(10*time.Second)))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	foundRetry := false
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "retry: 10000") {
			foundRetry = true
			break
		}
	}

	if !foundRetry {
		t.Error("expected initial retry hint in SSE stream")
	}
}

func TestSSE_StreamClosed(t *testing.T) {
	stream := &SSEStream{closed: atomic.Bool{}}
	stream.closed.Store(true)

	err := stream.Send(Event{Data: "test"})
	if err == nil {
		t.Error("expected error when sending to closed stream")
	}
}

// TestStream_PreFlightReject_404 asserts that a pre-flight rejection
// closes Barista F-02: the response must be a real HTTP 404 with the
// framework's structured JSON error envelope, NOT a 200-OK SSE stream
// containing an `event: error` frame.
func TestStream_PreFlightReject_404(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		t.Error("handler must not run when pre-flight rejects")
		return nil
	}

	preflight := func(ctx context.Context) error {
		return ErrNotFound("app not found").WithDetail("app_id", "missing")
	}

	router := Portafilter().Get("/stream",
		StreamSimple(handler, WithPreFlight(preflight)),
	)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}
	if strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected NO text/event-stream header; got %q", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if bytes.Contains(body, []byte("event: error")) {
		t.Errorf("body must not contain `event: error` frame: %s", body)
	}

	var envelope struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode JSON envelope: %v (body=%s)", err, body)
	}
	if envelope.Error.Code != "NOT_FOUND" {
		t.Errorf("expected error.code NOT_FOUND, got %q", envelope.Error.Code)
	}
	if envelope.Error.Message != "app not found" {
		t.Errorf("expected error.message 'app not found', got %q", envelope.Error.Message)
	}
	if got := envelope.Error.Details["app_id"]; got != "missing" {
		t.Errorf("expected error.details.app_id 'missing', got %v", got)
	}
}

// TestStream_PreFlightAccept_HeadersOK asserts that a pre-flight that
// returns nil leaves the existing stream flow unchanged: SSE headers
// commit and events are delivered.
func TestStream_PreFlightAccept_HeadersOK(t *testing.T) {
	preflightCalled := atomic.Bool{}

	preflight := func(ctx context.Context) error {
		preflightCalled.Store(true)
		return nil
	}

	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Name: "ok", Data: "accepted"})
	}

	router := Portafilter().Get("/stream",
		StreamSimple(handler, WithPreFlight(preflight)),
	)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !preflightCalled.Load() {
		t.Error("expected pre-flight closure to be invoked")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}

	events := parseSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Name != "ok" || events[0].Data != "accepted" {
		t.Errorf("expected event ok/accepted, got %s/%s", events[0].Name, events[0].Data)
	}
}

// TestStream_PreFlightHonorsCtx asserts the pre-flight closure receives
// the request context and can resolve router state via MustGetState[T].
func TestStream_PreFlightHonorsCtx(t *testing.T) {
	type authState struct {
		Allowed bool
	}

	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Name: "ok", Data: "go"})
	}

	preflight := func(ctx context.Context) error {
		s := MustGetState[authState](ctx)
		if !s.Allowed {
			return ErrForbidden("not allowed")
		}
		return nil
	}

	t.Run("allowed", func(t *testing.T) {
		router := Portafilter().
			WithState(authState{Allowed: true}).
			Get("/stream", StreamSimple(handler, WithPreFlight(preflight)))
		server := httptest.NewServer(router)
		defer server.Close()

		resp, err := http.Get(server.URL + "/stream")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		events := parseSSEEvents(t, resp)
		if len(events) == 0 || events[0].Data != "go" {
			t.Errorf("expected event data 'go', got %+v", events)
		}
	})

	t.Run("denied", func(t *testing.T) {
		router := Portafilter().
			WithState(authState{Allowed: false}).
			Get("/stream", StreamSimple(handler, WithPreFlight(preflight)))
		server := httptest.NewServer(router)
		defer server.Close()

		resp, err := http.Get(server.URL + "/stream")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
			t.Errorf("rejected stream must not advertise text/event-stream, got %q", ct)
		}
	})
}

// TestStream_NoPreFlight_BackwardCompat asserts that StreamSimple
// without the WithPreFlight option behaves byte-identical to v2.0: the
// same headers, status, and frame layout that TestSSE_BasicStream
// verifies.
func TestStream_NoPreFlight_BackwardCompat(t *testing.T) {
	handler := func(ctx context.Context, stream *SSEStream) error {
		return stream.Send(Event{Name: "message", Data: "hello"})
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %q", cc)
	}

	events := parseSSEEvents(t, resp)
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(events))
	}
	if events[0].Name != "message" || events[0].Data != "hello" {
		t.Errorf("expected event message/hello, got %s/%s", events[0].Name, events[0].Data)
	}
	if events[0].ID != "1" {
		t.Errorf("expected auto-assigned ID '1', got %q", events[0].ID)
	}
}

type sseEvent struct {
	ID    string
	Name  string
	Data  string
	Retry string
}

func parseSSEEvents(t *testing.T, resp *http.Response) []sseEvent {
	t.Helper()
	var events []sseEvent
	scanner := bufio.NewScanner(resp.Body)
	var current sseEvent

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current != (sseEvent{}) {
				events = append(events, current)
				current = sseEvent{}
			}
			continue
		}
		if strings.HasPrefix(line, ": ") {
			continue
		}
		if strings.HasPrefix(line, "id: ") {
			current.ID = strings.TrimPrefix(line, "id: ")
		} else if strings.HasPrefix(line, "event: ") {
			current.Name = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			current.Data = strings.TrimPrefix(line, "data: ")
		} else if strings.HasPrefix(line, "retry: ") {
			current.Retry = strings.TrimPrefix(line, "retry: ")
		}
	}

	if current != (sseEvent{}) {
		events = append(events, current)
	}

	return events
}
