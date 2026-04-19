# WebSocket Echo Example

Demonstrates WebSocket support in Espresso.

## Run

    go run ./cmd/example/websocket

## Test

Connect with wscat:

    wscat -c ws://localhost:8080/ws/lobby

Or use any WebSocket client to connect to `ws://localhost:8080/ws/{room}`.