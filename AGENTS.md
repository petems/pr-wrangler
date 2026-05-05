# Repository Guidelines

## Project Structure & Module Organization
Start with `README.md` for the product overview, local requirements, and config shape. `main.go` is the CLI entrypoint and wires config, GitHub access, tmux session management, session persistence, and the Bubble Tea UI together. Core code lives under `internal/`:

- `internal/config`: config loading and session-related config types
- `internal/github`: GitHub client, PR types, status/action logic
- `internal/tmux`: tmux and git worktree/session orchestration
- `internal/tui`: Bubble Tea model, commands, and styles
- `internal/session`: local session/history persistence

Keep tests next to the code they cover, using `*_test.go`. The `docs/` directory is available for design notes and supporting documentation.

## Build, Test, and Development Commands
- `make build`: compile `./pr-wrangler`
- `make run`: build and launch the TUI locally
- `make test`: run all Go tests
- `make test-race`: run tests with the race detector
- `make test-cover`: print package coverage summary
- `make lint`: run `golangci-lint`
- `make fmt`: run `go fmt` and `gofumpt` when installed
- `make check`: CI-style validation (`fmt-check`, `vet`, `test-race`)

Use `make help` to list the full target set.

## Coding Style & Naming Conventions
Follow standard Go style: tabs for indentation, exported names in `CamelCase`, unexported names in `camelCase`, and concise receiver names like `m` for models/managers. Prefer small package-focused files inside `internal/...` over large cross-cutting modules. Format code before submitting with `make fmt`; CI-friendly format validation is `make fmt-check`.

## Testing Guidelines
Use Go’s built-in `testing` package. Name tests `TestXxx` and keep them table-driven when multiple scenarios share setup. Favor unit tests around command runners, TUI message flow, and GitHub/tmux edge cases. Run `make test` before opening a PR and `make test-race` for concurrency-sensitive changes.

## Commit & Pull Request Guidelines
Use short, imperative commit subjects such as `Add worktree setup for PR sessions`. Keep commits focused and avoid mixing UI, tmux, and config refactors unless they are part of one change. PRs should explain the user-visible change, note risks or follow-up work, and list verification commands run (for example, `make build`, `make test`). Include screenshots or terminal captures when changing TUI behavior.

## Configuration & Environment Tips
The app depends on local `git`, `tmux`, and GitHub CLI-compatible access through the configured runner. Avoid hardcoding machine-specific paths; use config-driven repo locations and session storage paths.

## Cursor Cloud specific instructions

**Runtime dependencies**: Go 1.25+, `tmux`, and `git` are pre-installed. `golangci-lint` is installed to `$HOME/go/bin` via the update script; `$HOME/go/bin` is already on `PATH` via `~/.bashrc`.

**GitHub authentication**: The TUI requires a GitHub token. Set `GITHUB_TOKEN` or `GH_TOKEN` before running `./pr-wrangler`. In Cloud Agent VMs, `gh auth token` provides a token but it may have limited scopes (e.g. 403 on user fetch). The app still launches and displays its TUI; PR search results depend on token permissions.

**Running the app**: `make build && GITHUB_TOKEN=$(gh auth token) ./pr-wrangler` launches the Bubble Tea TUI. Use `./pr-wrangler version` or `./pr-wrangler help` for non-interactive checks. The TUI is an alt-screen terminal app; press `q` to quit.

**Standard dev commands**: See `## Build, Test, and Development Commands` above or run `make help`.
