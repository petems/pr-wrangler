# PR Wrangler

[![CI](https://github.com/petems/pr-wrangler/actions/workflows/ci.yml/badge.svg)](https://github.com/petems/pr-wrangler/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/petems/pr-wrangler)](https://goreportcard.com/report/github.com/petems/pr-wrangler)

<img width="506" height="356" alt="Loading Screen" src="https://github.com/user-attachments/assets/851017e4-3714-43af-8b67-27af88b5a706" />

PR Wrangler is a terminal UI for triaging and acting on GitHub pull requests. It talks to the GitHub API directly through a native Go client (`go-github`), classifies each PR into actionable states like `Fix CI`, `Address feedback`, or `Resolve conflicts`, and launches task-focused tmux sessions for follow-up work.

## Requirements

- Go 1.25+
- `tmux` and `git` on `PATH`
- A GitHub token — obtained via `pr-wrangler auth login` (OAuth device flow) or supplied through `GITHUB_TOKEN` / `GH_TOKEN`
- Optional: `golangci-lint`, `gofumpt`

`pr-wrangler` deliberately does **not** depend on the `gh` CLI. It manages its own token so it only ever holds the scopes it actually needs (`repo`).

## Authentication

`pr-wrangler` has its own auth subcommand backed by GitHub's OAuth device flow:

```bash
pr-wrangler auth login    # interactive device flow, stores a scoped token
pr-wrangler auth status   # show current token source and verify it
pr-wrangler auth logout   # remove the stored token
```

On first `auth login` you'll be asked for an OAuth App **Client ID**. To create one (one-time setup at https://github.com/settings/applications/new):

- Application name: anything (e.g. `pr-wrangler`)
- Homepage URL: any URL
- Authorization callback URL: `http://localhost` (unused by device flow)
- Check **Enable Device Flow**

Copy the Client ID and paste it when prompted. It's saved to `oauth_client_id` in your config file so you won't be asked again. You can also pre-set it via the `PR_WRANGLER_CLIENT_ID` environment variable or by editing the config directly.

### Token resolution order

When the TUI starts (or any subprocess like the Claude agent runs), the token is resolved in this order:

1. `GITHUB_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. Stored token at `~/.config/pr-wrangler/auth.json` (or the platform-specific equivalent via `os.UserConfigDir()`)

The stored file is created with `0600` permissions and contains the token, the granted scopes, the GitHub username, and a creation timestamp. `pr-wrangler` re-exports the resolved token to spawned agents via `GITHUB_TOKEN=...` so the same identity is used end-to-end.

## Development

```bash
make build
make run
make test
make check
```

`make run` builds `./pr-wrangler` and starts the Bubble Tea interface.

### CI Validation

Before submitting a pull request, run `make check` to ensure your changes pass all CI checks:

- **Format Check**: `make fmt-check` - Validates code formatting
- **Lint**: `make lint` - Runs golangci-lint with configured rules
- **Vet**: `make vet` - Runs go vet for static analysis
- **Test**: `make test-race` - Runs tests with race detector
- **Coverage**: Target is 80% code coverage (currently tracked as a warning)

The CI pipeline will automatically run these checks on all pull requests.

## Configuration

The app reads config from `~/.config/pr-wrangler/config.yaml` (or the platform-specific equivalent — see `os.UserConfigDir`). If the file is missing, built-in defaults are used and `pr-wrangler` will surface the resolved path on the loading screen.

Example:

```yaml
repo_base_dir: /Users/you/projects
service_label_prefix: service:
color_scheme: dracula
oauth_client_id: Iv1.xxxxxxxxxxxxxxxx   # optional; set by `pr-wrangler auth login`
views:
  - name: My PRs
    query: author:@me is:open
    default: true
agent_commands:
  fix-ci: "claude --permission-mode acceptEdits 'The CI checks are failing on this PR: {{pr_url}} - Investigate and fix the issues.'"
```

Files written to the config dir:

- `config.yaml` — application config (this file)
- `auth.json` — stored OAuth token (mode `0600`, written by `pr-wrangler auth login`)
- `history.json` — persisted PR session history

### Color Schemes

Set `color_scheme` to one of the following values (default: `default`):

| Value | Description |
|---|---|
| `default` | Classic dark-terminal green/cyan palette |
| `dracula` | Purple/pink accents inspired by the Dracula theme |
| `solarized` | Blue/cyan palette inspired by Solarized Dark |
| `nord` | Blue-gray/frost/aurora palette from the Nord theme |

### Query warnings

`pr-wrangler` warns when the configured search query has no `is:open`, `is:closed`, or `is:merged` qualifier — those queries can hit GitHub's 1000-result search cap and trip secondary rate limits. The configured query is shown above the table on the main dashboard, while the effective query (`<your query> is:pr`) is shown on the loading screen so you can confirm exactly what is being searched.

## Demo Mode

`pr-wrangler` ships with a built-in demo mode that renders the TUI from a fixed set of mock PRs, SAML errors, and tmux sessions. It does **not** call the GitHub API and does **not** require any authentication, so it's useful for screenshots, talks, or just kicking the tyres without a token.

```bash
pr-wrangler demo            # interactive TUI populated with mock data
pr-wrangler demo --render   # render one frame to stdout (ANSI colour codes preserved)
pr-wrangler demo -r         # short form of --render
```

For convenience the Makefile exposes the same flows plus image and animation capture:

```bash
make preview          # builds the binary and runs `pr-wrangler demo --render`
make preview-capture  # writes the rendered frame to preview.txt
make preview-image    # renders preview.png and preview.svg via freeze
make preview-gif      # renders demo.gif via VHS (uses demo.tape)
make preview-all      # all of the above in one shot
```

`preview-image` and `preview-gif` shell out to two charmbracelet tools that you can install ahead of time:

```bash
go install github.com/charmbracelet/freeze@latest
go install github.com/charmbracelet/vhs@latest
# or, on macOS:  brew install charmbracelet/tap/freeze vhs
```

The mock data covers a representative slice of states: open/draft PRs, passing/failing/pending CI, approved/changes-requested/commented reviews, mergeable/conflicting branches, and SAML-protected placeholders.

### Capturing screenshots and GIFs

There are three complementary capture flows, all driven by the same demo binary:

| Output | How | When to use |
|---|---|---|
| `preview.txt` | `pr-wrangler demo --render > preview.txt` | Fast text snapshot for diffs, PR comments, and agentic flows that consume ANSI directly. |
| `preview.png` / `preview.svg` | `make preview-image` (uses [`freeze`](https://github.com/charmbracelet/freeze)) | README hero images, blog posts, talk slides — anywhere a static image renders better than ANSI. |
| `demo.gif` | `make preview-gif` (uses [`vhs`](https://github.com/charmbracelet/vhs) with `demo.tape`) | Animated walkthrough for the README header and onboarding docs. |

Agents (Claude Code, Codex, etc.) can run any of the targets above and read the resulting files directly — `preview.png` is most reliable for visual verification because no ANSI-aware rendering is required.

## PR Preview Comments

The CI pipeline includes a `UI Preview` job that runs on every pull request. It:

1. Builds `pr-wrangler` from the PR branch.
2. Runs `pr-wrangler demo --render` to capture `preview.txt`.
3. Pipes that snapshot through `freeze` to produce `preview.png` and `preview.svg`.
4. Renders `demo.tape` through [`charmbracelet/vhs-action`](https://github.com/charmbracelet/vhs-action) to produce `demo.gif`.
5. Uploads `preview.txt`, `preview.png`, `preview.svg`, and `demo.gif` as the `ui-preview` workflow artifact.
6. Posts (or updates) a comment on the PR with the ANSI snapshot wrapped in an `ansi` fenced code block, plus a link to the workflow run where the image/animation artifacts live.

The comment is keyed by a hidden marker (`<!-- pr-wrangler:ui-preview -->`), so subsequent pushes update the existing comment instead of creating duplicates.

**Forked PRs**: GitHub downgrades `GITHUB_TOKEN` to read-only on `pull_request` runs from a forked head repo, so the comment step is gated with `if: github.event.pull_request.head.repo.full_name == github.repository`. Artifacts are still uploaded for fork PRs — reviewers can fetch them from the workflow run page even when the auto-comment is skipped.

## Repository Layout

- `main.go`: CLI entrypoint and `pr-wrangler auth` subcommand
- `internal/github`: native go-github client, OAuth device flow, PR classification, SAML error detection
- `internal/tmux`: tmux session and git worktree management
- `internal/tui`: Bubble Tea UI model, async commands, styles
- `internal/session`: persisted session history (`history.json`)
- `internal/config`: YAML config loading and defaults

## Workflow

1. Resolve a GitHub token via the chain documented in [Authentication](#authentication).
2. Run a single GitHub Search API call (`<query> is:pr`) to find candidate PRs.
3. Fan out up to 8 concurrent PR detail workers (each worker fetches `pulls`, `reviews`, `check-runs`, and combined status sequentially); PRs are reassembled in the original `updated-desc` order.
4. Detect 403 SAML errors (via `X-GitHub-SSO` header or known message body) and surface them inline as "SAML Auth Required" rows that can be authorized with `a`.
5. Classify each PR (`DetermineStatus` → `DetermineAction` in `internal/github`).
6. On `enter` / `c`, ensure a git worktree for the PR's head branch, then create or attach to a tmux session named after the PR, launching the configured agent command.

## Features

### SAML-Protected Repository Support

PR Wrangler gracefully handles repositories protected by organizational SAML authentication:

- **Automatic Detection**: When fetching PRs, any that return 403 SAML errors are automatically detected
- **Visible in TUI**: SAML-protected PRs appear in the list with status "SAML Auth Required" and action "Authorize SAML"
- **One-Click Authorization**: Press `a` on a selected SAML-protected PR to open the authorization URL in your browser
- **Graceful Degradation**: Successfully loaded PRs are displayed alongside SAML-protected ones, allowing you to work with accessible PRs while handling authorization separately

**Usage Flow:**
1. Launch PR Wrangler - SAML-protected PRs will appear with "SAML Auth Required" status
2. Select a SAML-protected PR and press `a` to authorize
3. Complete the SAML authentication flow in your browser
4. Press `r` to refresh - the PR will now load successfully

### Keyboard Shortcuts

- `q` / `ctrl+c`: Quit
- `r`: Refresh PR list
- `enter` / `c`: Open or switch to Claude session for selected PR
- `o`: Open selected PR in browser
- `a`: Authorize SAML access for selected PR (opens authorization URL)
- `?`: Toggle help
