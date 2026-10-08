# Contributing to TailLaunch

Thanks for helping improve TailLaunch. Keep changes focused and explain the
user problem or maintenance need they address.

## Before you start

Read the [Getting Started](docs/getting-started.md),
[Development and Testing](docs/development.md), and [Security Model](docs/security-model.md)
docs. Do not open a public issue or pull request with vulnerability details;
use [SECURITY.md](SECURITY.md).

## Local workflow

Install Go `1.26+`, make the requested change, and run the checks relevant to
it:

```bash
go test ./...
go test -tags stub ./...
go vet ./...
```

If available, also run `staticcheck ./...` and `govulncheck ./...`. User-facing
changes should include documentation updates. GUI interaction checks require a
desktop environment; do not claim them as covered by headless tests.

## Pull requests

Use the pull-request template. Include a concise summary, validation commands,
platform limitations, and screenshots when a visual flow changes. Keep
generated binaries, persistent Tailscale state, browser profiles, credentials,
and private tailnet data out of commits.

## Scope and review

Reviewers will prioritize correctness, clear failure behavior, privacy,
cross-platform compatibility, and maintainable CI. Repository documentation is
canonical; do not create or update GitHub Wiki pages as part of a change.
