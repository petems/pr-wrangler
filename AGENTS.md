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

The app depends on local `git`, `tmux`, and a GitHub API token. The token is resolved in order from `GITHUB_TOKEN`, `GH_TOKEN`, then a stored OAuth token written by `pr-wrangler auth login`. Do not reintroduce a hard dependency on the `gh` CLI for API calls — the project deliberately uses the native `go-github` client so that scopes (`repo`) are explicit and bounded. The resolved token is re-exported as `GITHUB_TOKEN` to any subprocess (e.g. the Claude agent) so the same identity is used end-to-end. Avoid hardcoding machine-specific paths; use config-driven repo locations and session storage paths.

## GitHub Integration

The GitHub layer lives in `internal/github`:

- `client.go` wraps `go-github` v72. `FetchPRs` runs the Search API once, then fans out up to 8 concurrent PR detail workers (each worker fetches its pull, reviews, check runs, and combined status sequentially) and reassembles results in original search order so the UI ordering is stable.
- `device_flow.go` implements the OAuth device-flow handshake against `github.com/login/device/code` and `github.com/login/oauth/access_token`. No client secret is required; only a public OAuth App Client ID with "Device Flow" enabled.
- `auth.go` stores `TokenInfo` at `~/.config/pr-wrangler/auth.json` with mode `0600`, and exposes `ResolveToken()` for the env-var → stored-token chain.
- `SAMLAuthError` / `SAMLErrorEntry` represent 403 SAML failures. They are interleaved back into the PR list at their original index by the TUI so SAML-protected PRs stay in the right position rather than being silently dropped or appended.

When changing the fetch flow, preserve: (1) original search order, (2) the per-PR concurrency cap, (3) SAML detection via both the `X-GitHub-SSO` header and the documented message body, and (4) the progress callback contract (`progress(0, total)` after search, then `progress(done, total)` per completion).

## TUI Message Flow

The Bubble Tea model in `internal/tui/model.go` follows a deliberate async pattern:

- A fetch returns `prsFetchStartedMsg` with a channel; the model then drains `prsProgressMsg` events and finally a `prsLoadedMsg`. Every message is tagged with the source channel so messages from a superseded fetch (e.g. user pressed `r` mid-fetch) are dropped instead of corrupting state — keep that invariant when adding new fetch-shaped commands.
- Opening a session is a three-step chain: `ensureWorktreeCmd` → `worktreeReadyMsg` → `ensureSessionCmd` → `sessionReadyMsg` → `switchClientCmd` (which uses `tea.ExecProcess` to suspend Bubble Tea before handing the terminal to `tmux switch-client` / `attach-session`). If you add new pre-session work, slot it before `ensureSessionCmd`; do not block the Update loop on tmux I/O.

## Agentic UI/UX Validation

When you change anything that affects what the TUI looks like — styles, table layout, key handling, status/action labels, the loading screen, the help overlay, the theme picker, or any new visual affordance — you must validate it visually as well as programmatically. The demo mode exists exactly for this.

**Programmatic validation** (always do this first):

```bash
make check        # fmt-check + lint + vet + test-race
```

**Visual validation** — pick the capture format that matches the change:

| Capture                     | Command                | Use it for                                                                                                               |
| --------------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `preview.txt` (ANSI text)   | `make preview-capture` | Quick diffing, regression tracking, anything where ANSI escape sequences are the source of truth.                        |
| `preview.png` (still image) | `make preview-image`   | Layout, alignment, colour palette, table column widths, overlay positioning — anywhere a still frame conveys the change. |
| `demo.gif` (animated)       | `make preview-gif`     | Anything time-based: spinner behaviour, progress bar, theme picker selection flow, key-driven state transitions.         |
| `make preview-all`          | runs all three         | Substantial UI changes where you want every artifact in one shot.                                                        |

The `preview-image` and `preview-gif` targets shell out to [`freeze`](https://github.com/charmbracelet/freeze) and [`vhs`](https://github.com/charmbracelet/vhs) respectively. If they're not installed, the targets fail fast with the install hint:

```bash
go install github.com/charmbracelet/freeze@latest
go install github.com/charmbracelet/vhs@latest
```

**If `make preview-image` segfaults on Linux** (`SIGSEGV` / `runtime.memmove` inside `freeze`), install `librsvg2-bin`. `freeze`'s embedded WASM SVG → PNG rasteriser crashes intermittently; when `rsvg-convert` is on `PATH`, `freeze` prefers it and bypasses the WASM renderer entirely. Pair it with `fonts-jetbrains-mono` so the rasterised PNG keeps the column alignment `freeze` declares in its SVG output:

```bash
sudo apt-get update
sudo apt-get install -y librsvg2-bin fonts-jetbrains-mono
fc-cache -f
```

See [charmbracelet/freeze#203](https://github.com/charmbracelet/freeze/issues/203) for background. CI installs both packages for the same reason — see the `Install freeze` step in `.github/workflows/ci.yml`.

**For agents (Claude Code, Codex, Cursor, etc.):**

1. Run `make preview-image` and read the resulting `preview.png` directly — your multimodal Read tool will render it as an image, which is the most reliable way to verify visual output without an ANSI-aware terminal.
2. For interaction-driven changes, edit `demo.tape` to exercise the new flow, then run `make preview-gif` and inspect `demo.gif` (or extract a specific frame with `ffmpeg -i demo.gif -vf "select=eq(n\,N)" -vframes 1 frame.png`).
3. Keep the demo data honest. If you add a new PR status, action, or row type, extend the mock fixtures in `internal/tui/mockdata.go` so the demo continues to exercise every branch of the renderer.
4. CI runs the same capture pipeline on every PR and posts the ANSI snapshot back as a comment (with PNG/SVG/GIF attached as artifacts), so your local previews and the PR review surface should match.

**Reporting back to the user**: if you're claiming a UI change works, attach or reference the captured artifact rather than describing it in prose — "preview.png shows the new column rendering correctly at 140 cols" is verifiable; "I added the column" is not.

## Cursor Cloud specific instructions

**Runtime dependencies**: Go 1.25+, `tmux`, and `git` are pre-installed. `golangci-lint` is installed to `$HOME/go/bin` via the update script; `$HOME/go/bin` is already on `PATH` via `~/.bashrc`.

**GitHub authentication**: The TUI requires a GitHub token. The preferred path is `./pr-wrangler auth login` (OAuth device flow) which stores a scoped token in `~/.config/pr-wrangler/auth.json`. Alternatively, set `GITHUB_TOKEN` or `GH_TOKEN` before running `./pr-wrangler`. In Cloud Agent VMs, `gh auth token` provides a fallback but it may have limited scopes (e.g. 403 on user fetch). The app still launches and displays its TUI; PR search results depend on token permissions.

**Running the app**: `make build && ./pr-wrangler` launches the Bubble Tea TUI (assumes you've already run `./pr-wrangler auth login` or exported `GITHUB_TOKEN`). Use `./pr-wrangler version` or `./pr-wrangler help` for non-interactive checks. The TUI is an alt-screen terminal app; press `q` to quit.

**Standard dev commands**: See `## Build, Test, and Development Commands` above or run `make help`.
