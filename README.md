# Matrix-Nomadtable Bridge

A Matrix bridge that connects Matrix to Nomadtable (a Stream Chat-based service) using the mautrix-go bridgev2 framework.

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

## Requirements

- Go 1.25.5 or later
- libolm (for crypto features): `brew install libolm` on macOS
