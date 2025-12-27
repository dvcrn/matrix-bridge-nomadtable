# Matrix-Nomadtable Bridge

A Matrix bridge that connects Matrix to Nomadtable (a Stream Chat-based service) using the mautrix-go bridgev2 framework.

## Project Components

### Nomadtable Client Library

The `pkg/nomadtable/` directory contains a Go client library for interacting with the Nomadtable API:

- **`client.go`** - REST API client with methods for:
  - `UpdateUser` - update user info
  - `GetApp` - get app settings
  - `QueryChannels` - list channels for a user
  - `QueryChannel` - get messages for a specific channel

- **`websocket.go`** - WebSocket client featuring:
  - `ConnectWebsocket` method that connects to Stream's `/connect` endpoint
  - Automatic ping/pong handling (WebSocket-level and application-level)
  - Sends `health.check` JSON payload every 30s
  - Logging hooks via `WebsocketOptions.Logger`
  - Auto-extracts `connection_id` from first frame

- **`responses.go`** - Response type definitions
- **`events.go`** - WebSocket event types

### Matrix Bridge

The root directory contains the Matrix bridge implementation:

- **`main.go`** - Entry point using `mxmain` for bridge lifecycle
- **`network_connector.go`** - Core `bridgev2.NetworkConnector` implementation
- **`login.go`** - Login flow definitions (`bridgev2.LoginProcess`)
- **`network_client.go`** - Per-user network client

### Scripts

Example scripts demonstrating the Nomadtable client:

1. **`scripts/nomadtable_client_smoketest/main.go`** - Lists all channels/rooms for a user
2. **`scripts/nomadtable_ws_connect/main.go`** - Streams WebSocket frames with logging
3. **`scripts/nomadtable_channel_messages/main.go`** - Fetches messages for a specific channel

All scripts auto-connect to WebSocket to obtain `connection_id` if not provided via environment variables.

## Dependencies

- `github.com/gorilla/websocket` v1.5.3 - WebSocket client
- `github.com/google/uuid` v1.6.0 - UUID generation
- `maunium.net/go/mautrix` - Matrix bridge framework
- `github.com/rs/zerolog` - Logging

## Building and Running

### Build the Bridge

```bash
go build
```

### Generate Registration File

```bash
go run . -g -c config.yaml -r registration.yaml
```

### Run the Bridge

```bash
./matrix-nomadtable -c config.yaml -r registration.yaml
```

## Configuration

1. Generate `registration.yaml` using the `-g` flag
2. Configure `config.yaml` with:
   - `homeserver.address` and `homeserver.domain`
   - Copy `id`, `as_token`, `hs_token` from generated `registration.yaml`
   - Configure database path and permissions
3. Place `registration.yaml` in your homeserver config directory and restart

## Environment Variables

Scripts use `fnox` for secret management. Required secrets:

- `NOMADTABLE_API_KEY` - API key for Nomadtable
- `NOMADTABLE_AUTH` - Auth token
- `NOMADTABLE_USER_ID` - User ID

Run scripts with: `fnox x -- go run ./scripts/<script-name>/main.go`

## Development

See [AGENTS.md](./AGENTS.md) for detailed development guidelines and patterns.

## Requirements

- Go 1.25.5 or later
- libolm (for crypto features): `brew install libolm` on macOS
