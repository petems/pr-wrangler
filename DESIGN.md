# pr-wrangler TUI Design Baseline

## Purpose & Scope

This document pins the current `pr-wrangler` TUI as the design baseline and
states the rules future contributors should follow when changing it. It
describes the screen, the palette, the interaction contract, the accessibility
constraints, and the validation pipeline.

It is **not** a style guide for the README, a Go API reference, or a roadmap.
For product framing, see [`README.md`](./README.md); for codebase guidelines
and the async message-flow contract in agent terms, see
[`CLAUDE.md`](./CLAUDE.md).

## Product Context

`pr-wrangler` is for open-source maintainers and engineers who work across many
repositories and keep several pull requests in flight at once. They use it to
scan PR state quickly, understand what needs attention, and launch focused
agentic fixes or review follow-up sessions without leaving the terminal.

The product personality is fast, playful, and confident. It should feel like a
capable terminal operations cockpit with a deliberately silly "PR Wrangler"
streak: talking bull moments, lasso references, Western-flavored loading states,
and fun motion are welcome when they support the workflow.

Good references are agentic CLI tools and playful terminal apps such as
`agent-of-empires`: useful first, but willing to be memorable.

## Design Intent

`pr-wrangler` is a terminal-first PR operations cockpit. It exists to make
scanning, triage, and jumping into focused tmux sessions fast. The TUI is
dense, keyboard-driven, and optimized for someone who lives in the table.

The cowsay / "wrangler" Western motif is allowed as **chrome** around the
workflow — the loading banner, the dashboard header, the empty state. It
must never appear inside the data table or in any region where someone is
reading PR information to make a decision.

When in doubt: legibility beats personality.

## Information Hierarchy

The screen has a fixed render order. Future regions slot into this order
rather than inventing new layers.

**Loading screen** (transient, until the first `prsLoadedMsg`)

1. Block-letter banner
2. Centered cowsay loading message
3. Spinner + progress bar (`done / total`)
4. Config-source label, bottom-left

**Dashboard** (persistent)

1. Cowsay header + title
2. Metadata line showing sort mode, configured query, and active search
3. Query warning line (only when the resolved query differs from the user's
   expectation)
4. PR table — fixed columns plus a flexible title column
5. Page indicator
6. Error line (only when `lastError != nil`)
7. Notification line (transient; cleared on next state change)
8. Help line or active `/` search prompt

**Overlays** (rendered on top, intercept input)

- Expanded help (`?`)
- Theme picker (`t`)

Overlays are mutually exclusive. New overlays must follow the same precedence
rule: while an overlay is open, key input is routed to the overlay first and
the dashboard does not handle it.

## Interaction Principles

- **Keyboard-first.** Every action has a visible binding in the one-line
  help footer and in the `?` overlay. Mouse handling is not a goal.
- **Priority-first triage.** The default table order is action priority:
  fix CI, rebase/resolve conflicts, address feedback or review comments,
  approved mergeable PRs, then waiting/open/draft work. Users can cycle to
  non-priority sort orders with `s`, including newest and oldest updated PRs.
- **Search narrows the work queue.** `/` opens a Vim-style search prompt that
  filters visible rows by repo, PR number, title, branch, status, or action.
- **Stable selection.** Refresh (`r`) preserves the selected row when
  possible; this is part of the `prsLoadedMsg` contract.
- **Async work belongs in `tea.Cmd`.** The `Update` loop must never block.
  Long-running work returns a command; the model only reacts to typed
  messages — `prsFetchStartedMsg`, `prsProgressMsg`, `prsLoadedMsg`,
  `worktreeReadyMsg`, `sessionReadyMsg`, etc.
- **`View` is pure.** No side effects, no I/O, no state mutation. Rendering
  is a function of model state.
- **Overlays own input.** When `showHelp` or `showThemePicker` is true the
  overlay handles keys; the dashboard does not.

## Visual System

`pr-wrangler` uses semantic palette **tokens**, not raw hex. The tokens come
from `ColorScheme` in [`internal/tui/styles.go`](./internal/tui/styles.go):

| Token        | Role                                               |
| ------------ | -------------------------------------------------- |
| `Primary`    | Title, banner, selection indicator                 |
| `Secondary`  | Loading spinner, progress bar, help section heads  |
| `Error`      | Error messages, failed-CI status and fix-CI action |
| `Warning`    | Warnings, query banner                             |
| `Success`    | Approved/merged states and merge actions           |
| `Info`       | Waiting/open/investigate/SAML informational states |
| `Review`     | Review-comment and feedback states/actions         |
| `Conflict`   | Merge-conflict states/actions                      |
| `Draft`      | Draft states                                       |
| `Help`       | Help text, muted labels                            |
| `Border`     | Table border and structural chrome                 |
| `Header`     | Table header text                                  |
| `Repo`       | Repository column text                             |
| `Number`     | PR number column text                              |
| `TitleText`  | PR title column text                               |
| `SelectedBg` | Background of the selected row                     |
| `TableText`  | Base table foreground                              |

Themes are listed in `ThemeNames`: `default`, `dracula`, `solarized`, `nord`.
Each entry in `colorSchemes` must populate every token.

**Rules:**

- New visual elements consume tokens through `Styles` (see `NewStyles` in
  `internal/tui/styles.go`). Never reference raw hex from outside
  `styles.go`.
- New themes must populate every field and must be checked through the
  preview pipeline before they ship — including a contrast pass against the
  `Help` and `Error` colors, which are the easiest to mis-tune.
- Column color is structural, not decorative. The table is intentionally
  quiet: repository and date columns recede, PR number and title anchor the
  row, and the strongest semantic colors are reserved for status and action.
  Text labels remain the source of truth.
- Bold is a primary cue, not a decoration. Reserve it for selection,
  banners, and section heads.

## Layout & Responsiveness

- Fixed columns reserve roughly 110 cols of horizontal space; the title
  column absorbs the remainder via `titleColumnWidth()` in
  [`internal/tui/model.go`](./internal/tui/model.go).
- Page size is derived from terminal height. `pageBounds()` keeps the
  selected row inside the visible window.
- Rendered strings are measured with `lipgloss.Width` and truncated with
  `github.com/charmbracelet/x/ansi.Truncate` so OSC-8 hyperlinks and style
  escapes are not split mid-sequence.
- Re-flow happens only on `tea.WindowSizeMsg`. No layout work belongs
  elsewhere.

## Accessibility & Terminal Constraints

- **Color is never the only cue.** Every status and action is also a
  textual label — `Approved`, `Fix CI`, `Resolve conflicts`,
  `Address feedback`. A reader on a monochrome terminal must still be able
  to triage.
- **Contrast targets.** Aim for WCAG-style contrast when choosing new theme
  colors: ≥ 4.5:1 for body text, ≥ 3:1 for large or bold text. Dark-on-dark
  themes (`dracula`, `nord`) are the easiest to get wrong — verify against
  common color-vision-deficiency palettes before adding new colors there.
- **`NO_COLOR` is honored.** Lip Gloss v2's color profile detection handles
  this automatically when styles go through `Styles`. Do not bypass it by
  emitting raw ANSI escapes.
- **Hyperlinks are an enhancement.** OSC-8 hyperlinks are allowed but the
  surrounding text must remain readable when a terminal strips them.
- **ASCII art is chrome only.** The block-letter banner and the cowsay
  header are decorative. Never render PR data through ASCII art: it breaks
  at narrow widths and is hostile to screen readers.

## State & Message-Flow Contract

Two invariants from `CLAUDE.md` are part of the design, not just the
codebase:

**PR fetch.** Each fetch returns `prsFetchStartedMsg` carrying a channel.
The model drains `prsProgressMsg` events from that channel and finishes on
`prsLoadedMsg`. Every message is tagged with its source channel so messages
from a superseded fetch (e.g. the user pressed `r` mid-fetch) are dropped
instead of corrupting state. New fetch-shaped commands follow the same
superseded-channel discard rule.

**Session open.** Opening a session is a three-step chain:
`ensureWorktreeCmd` → `worktreeReadyMsg` → `ensureSessionCmd` →
`sessionReadyMsg` → `switchClientCmd`, which uses `tea.ExecProcess` to
suspend Bubble Tea before handing the terminal to `tmux switch-client` /
`attach-session`. New pre-session work slots in before `ensureSessionCmd`
and must not block the `Update` loop on tmux I/O.

## Demo Mode Is Part of the Design

[`internal/tui/mockdata.go`](./internal/tui/mockdata.go) exercises every
visible branch of the renderer: every PR status, every action label, drafts,
SAML-error placeholders, owned vs external authorship, active tmux sessions.

**Rule:** if a UI change adds a new visible state, the demo fixtures are
extended in the same PR. A new status that does not appear in the demo is
a regression in waiting.

## Validation Standard

Programmatic validation is non-negotiable:

```bash
make check        # fmt-check + lint + vet + test-race
```

Visual validation — pick the artifact that matches the change:

| Target                 | Output        | Use it for                                            |
| ---------------------- | ------------- | ----------------------------------------------------- |
| `make preview-capture` | `preview.txt` | ANSI diffs, regression tracking                       |
| `make preview-image`   | `preview.png` | Layout, alignment, column widths, overlay positioning |
| `make preview-gif`     | `demo.gif`    | Spinner, progress, theme picker flow, key-driven UX   |
| `make preview-all`     | all three     | Substantial UI changes                                |

When claiming a UI change works, attach the captured artifact to the PR
rather than describing the outcome in prose. Agents must read `preview.png`
back through their multimodal Read tool, not paraphrase it.

## Out of Scope for This Document

- Non-TUI surfaces: config file layout, OAuth flow, tmux session naming.
- Product roadmap and feature requests.
- Refactoring `internal/tui` to match this doc. If anything here is
  misaligned with the code today, that is a future PR — this document is
  the baseline.
