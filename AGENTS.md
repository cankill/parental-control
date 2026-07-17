# Repository Guidelines

## Project Structure & Module Organization

This is a Go 1.24 module for a macOS parental-control service. Executables live under `cmd/`: `parental-control` is the user-facing agent and `parental-control-helper` is the privileged Unix-socket helper. Application code is grouped by responsibility in `internal/`, including `bot`, `browser`, `helper`, `media`, and `statistics`. Shared configuration, storage, and domain types are under `internal/lib/`. Tests sit beside the code they cover as `*_test.go`. Runtime data may appear in `database/`; generated binaries belong in `out/` and should not be committed.

## Build, Test, and Development Commands

- `go mod download` installs the versions pinned in `go.mod` and `go.sum`.
- `go build -o out/parental-control ./cmd/parental-control` builds the main macOS application.
- `go build -o out/parental-control-helper ./cmd/parental-control-helper` builds the privileged helper.
- `go test ./...` runs the complete test suite.
- `go test -race ./...` checks concurrent code for data races.
- `go vet ./...` performs standard static checks.

Run and build on macOS: the project uses Darwin frameworks and a Darwin-specific authorization implementation.

## Coding Style & Naming Conventions

Format every changed Go file with `gofmt -w`. Follow standard Go conventions: tabs for indentation, short lowercase package names, exported identifiers in `PascalCase`, and unexported identifiers in `camelCase`. Keep packages focused and place reusable behavior in `internal/`, not in command entry points. Name files by responsibility, such as `domaintracker.go`; use `_darwin.go` or build tags for platform-specific code. Return or wrap errors with useful operation context.

## Testing Guidelines

Use Go's `testing` package and name tests `TestBehavior`, colocated with the package under test. Prefer table-driven tests for multiple inputs and `t.TempDir()` for filesystem cases. Add regression tests for fixes, especially around storage, IPC validation, browser parsing, and time boundaries. No coverage threshold is enforced, but new branches should be exercised.

## Commit & Pull Request Guidelines

History uses concise imperative subjects, often with prefixes such as `feat:` and `fix:`. Prefer `feat: add domain limit` or `fix: validate helper request`, keeping each commit focused. Pull requests should explain behavior and motivation, identify macOS or privilege implications, link relevant issues, and report `go test ./...` results. Include logs or screenshots when bot output or visible behavior changes.

## Security & Configuration

Set `PARENTAL_CONTROL_ENV` to an environment file containing `TG_BOT_TOKEN`; optional values include `TG_ADMIN_IDS` and `URL_POLL_SECONDS`. Never commit tokens, local environment files, generated databases, or credentials. Treat helper socket permissions and `/etc/hosts` changes as security-sensitive and preserve peer-credential and domain-whitelist checks.
