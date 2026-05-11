# PR Wrangler

[![CI](https://github.com/petems/pr-wrangler/actions/workflows/ci.yml/badge.svg)](https://github.com/petems/pr-wrangler/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/petems/pr-wrangler)](https://goreportcard.com/report/github.com/petems/pr-wrangler)

<img width="506" height="356" alt="Loading Screen" src="https://github.com/user-attachments/assets/851017e4-3714-43af-8b67-27af88b5a706" />

PR Wrangler is a terminal UI for triaging and acting on GitHub pull requests. It uses the `gh` CLI to discover PRs, classifies each PR into actionable states like `Fix CI`, `Address feedback`, or `Resolve conflicts`, and launches task-focused tmux sessions for follow-up work.

## Requirements

- Go 1.25+
- `gh` authenticated against GitHub
- `tmux`
- Optional: `golangci-lint`, `gofumpt`

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

The app reads config from `~/.config/pr-wrangler/config.yaml`. If the file is missing, built-in defaults are used.

Example:

```yaml
repo_base_dir: /Users/you/projects
service_label_prefix: service:
color_scheme: dracula
views:
  - name: My PRs
    query: author:@me
    default: true
agent_commands:
  fix-ci: "claude --permission-mode acceptEdits 'The CI checks are failing on this PR: {{pr_url}} - Investigate and fix the issues.'"
```

### Color Schemes

Set `color_scheme` to one of the following values (default: `default`):

| Value | Description |
|---|---|
| `default` | Classic dark-terminal green/cyan palette |
| `dracula` | Purple/pink accents inspired by the Dracula theme |
| `solarized` | Blue/cyan palette inspired by Solarized Dark |
| `nord` | Blue-gray/frost/aurora palette from the Nord theme |

Session history is stored at `~/.config/pr-wrangler/history.json`.

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

- `main.go`: CLI entrypoint
- `internal/github`: GitHub CLI integration and PR classification
- `internal/tmux`: tmux and git worktree/session management
- `internal/tui`: Bubble Tea UI model, commands, and styling
- `internal/session`: persisted session history
- `internal/config`: config loading and defaults

## Workflow

1. Fetch PRs from GitHub search.
2. Enrich each PR with `gh pr view`.
3. Classify the next likely action.
4. Launch or reuse a tmux session for the selected PR.
5. Open work in a dedicated repo checkout/worktree when available.

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
