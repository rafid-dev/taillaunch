# Development and Testing

## Requirements

- Go `1.26+` as specified by `go.mod`.
- A supported Chromium-family browser for manual launcher checks.

## Build

```bash
go build -trimpath -o taillaunch ./cmd/taillaunch
go build -trimpath -o taillaunch-gui ./cmd/taillaunch-gui
```

The `stub` build tag avoids downloading/building the live Tailscale backend for
local tests that do not require a real tailnet:

```bash
go test -tags stub ./...
```

## Quality checks

Run the same focused checks used by CI when the tools are installed:

```bash
go test ./...
go test -tags stub ./...
go vet ./...
staticcheck ./...
govulncheck ./...
```

Do not include Tailscale credentials, persistent state, browser profiles, or
private hostnames in commits, logs, screenshots, or issue reports.

## Pull requests

Keep changes focused, update user-facing documentation when behavior changes,
and explain testing in the pull request. Runtime architecture changes are out
of scope for documentation-only or repository-hardening work. See
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Releases

The release workflow builds CLI and GUI artifacts for Windows, Linux, and
macOS on `amd64` and `arm64`, then publishes a combined `SHA256SUMS.txt`.
Release tags are intentionally not created or pushed by local development
checks.
