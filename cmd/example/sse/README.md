# SSE Time Stream Example

Demonstrates Server-Sent Events streaming in Espresso.

## Run

    go run ./cmd/example/sse

## Test

Connect with curl:

    curl -N http://localhost:8080/stream

Or use any SSE client to connect to `http://localhost:8080/stream`.