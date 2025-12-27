# Agent Memory & Guidelines

This file serves as a persistent memory and rulebook for AI agents working on this project. It should be updated whenever new patterns, constraints, or decisions are discovered.

## Technology Stack & Idioms

- **Go Version**: The project uses Go 1.25.5.
  - We can and should use the `any` keyword as an alias for `interface{}`.

## Project Structure & Patterns

### Nomadtable Client Library

- **API Responses**: Response structs are defined in `pkg/nomadtable/responses.go` rather than being inline or in `client.go`.
  - Nested objects must be pointers.
- **API Client**: REST client implementation is in `pkg/nomadtable/client.go`.
- **Websocket Client**: Websocket connection + keepalive logic is in `pkg/nomadtable/websocket.go`.

### Matrix Bridge Structure

- `main.go` is the entry point; it wires config, logging, and bridge lifecycle (`mxmain`).
- `network_connector.go` implements the core `bridgev2.NetworkConnector` logic.
- `login.go` defines login flows (`bridgev2.LoginProcess`).
- `network_client.go` is the per-user remote network client.
- `go.mod`/`go.sum` manage dependencies; no dedicated `tests/` directory yet.

## Nomadtable Usage Recipes

### List channels (rooms) for a user

Use `QueryChannels` (requires `user_id` and `connection_id`). If you don't have a `connection_id`, open a websocket first and read the first frame to get it.

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

## Build, Test, and Development Commands

- `go run . -g -c config.yaml -r registration.yaml` generates `registration.yaml`.
- `go run . -c config.yaml -r registration.yaml` runs the bridge from source.
- `go build` builds the binary (default name `minibridge`).
- `go test ./...` runs tests (currently no test files).
- Toolchain: `mise install` and `mise exec go@1.25.5 -- <cmd>`.

## Coding Style & Naming Conventions

- Go code follows `gofmt` (tabs; standard Go formatting).
- Filenames use snake_case (e.g., `network_client.go`).
- Keep new files in the repo root unless you add a new package.

## Bridge Build & Operation Guide

- Implement the network side in `network_connector.go`: `GetName`, `GetCapabilities`, `GetLoginFlows`, `CreateLogin`, `LoadUserLogin`, `Start`, `Stop`.
- If you need network-specific settings, add a config file (e.g., `simple-config.yaml`) and load it in `GetConfig()`.
- Configure `config.yaml` with `homeserver.address`, `homeserver.domain`, database path, and permissions.
- Copy `id`, `as_token`, `hs_token`, and bot settings from `registration.yaml` into `config.yaml`.
- Place `registration.yaml` into your homeserver config directory and restart the homeserver.

## Portal Rooms vs Direct Rooms

- **Direct Matrix rooms** (`c.bridge.Bot.CreateRoom`) are not routed through the bridge and will not trigger `HandleMatrixMessage`.
- **Portal rooms** (`portal.CreateMatrixRoom`) are registered in the bridge DB and do trigger routing.
- When `HandleMatrixMessage` is not firing, ensure the room is created as a portal for the remote room ID.

## Testing Guidelines

- Use standard Go tests (`*_test.go`, `TestXxx`).
- Run `go test ./...` before committing.

## Commit & Pull Request Guidelines

- Commit messages are short, imperative summaries (e.g., "Add login flow").
- Keep commits focused; avoid mixing refactors with behavior changes.
- PRs should explain what changed and why, and link issues when applicable.

## Security & Configuration Tips

- libolm is required for crypto features; on macOS: `brew install libolm`.
- Avoid committing credentials; use environment variables or local config files.
