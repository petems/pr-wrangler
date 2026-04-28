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
views:
  - name: My PRs
    query: author:@me
    default: true
agent_commands:
  fix-ci: "claude --permission-mode acceptEdits 'The CI checks are failing on this PR: {{pr_url}} - Investigate and fix the issues.'"
```

Session history is stored at `~/.config/pr-wrangler/history.json`.

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
