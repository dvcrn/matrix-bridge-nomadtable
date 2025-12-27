# Agent Memory & Guidelines

This file serves as a persistent memory and rulebook for AI agents working on this project. It should be updated whenever new patterns, constraints, or decisions are discovered.

## Technology Stack & Idioms

- **Go Version**: The project uses Go 1.25.5.
  - We can and should use the `any` keyword as an alias for `interface{}`.

## Project Structure & Patterns

- **API Responses**: Response structs are defined in `pkg/nomadtable/responses.go` rather than being inline or in `client.go`.
  - Nested objects must be pointers.
- **API Client**: REST client implementation is in `pkg/nomadtable/client.go`.
- **Websocket Client**: Websocket connection + keepalive logic is in `pkg/nomadtable/websocket.go`.

## Nomadtable Usage Recipes

### List channels (rooms) for a user

Use `QueryChannels` (requires `user_id` and `connection_id`). If you don’t have a `connection_id`, open a websocket first and read the first frame to get it.

- Script: `scripts/nomadtable_client_smoketest/main.go`
- Code:
  - `client := nomadtable.NewClient(apiKey, authToken)`
  - `resp, err := client.QueryChannels(ctx, userID, connectionID, req)`

### Query messages for a channel

Use `QueryChannel` for a single channel. This returns `messages` for that channel.

- Script: `scripts/nomadtable_channel_messages/main.go`
- Code:
  - `resp, err := client.QueryChannel(ctx, channelType, channelID, userID, connectionID, &nomadtable.QueryChannelRequest{State: true, Watch: false, Presence: false})`
  - Iterate `resp.Messages`.

### Connect websocket and handle events

Use `ConnectWebsocket` to establish a websocket connection that continuously streams frames.

- `msgs := make(chan nomadtable.WebsocketMessage, 256)`
- `session, err := client.ConnectWebsocket(ctx, userID, msgs, &nomadtable.WebsocketOptions{Logger: logFn})`

To bind a handler, run a goroutine (or loop) that receives from the channel and unmarshals JSON:

- `for msg := range msgs { /* json.Unmarshal(msg.Data, &nomadtable.WebSocketEvent) ... */ }`

Notes:
- `session.ConnectionID()` is populated from incoming frames (first `health.check` usually contains it).
- The client sends websocket pings and application-level `health.check` frames periodically; the server replies with `health.check` events.
