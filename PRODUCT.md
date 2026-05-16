## Design Context

### Users

`pr-wrangler` is for open-source maintainers and engineers working across many
repositories who routinely have several pull requests in flight. They need to
scan PR state quickly, understand what needs attention, and launch focused
agentic fixes or review follow-up sessions without leaving their terminal.

### Brand Personality

Fast, playful, and confident. The interface should feel like a capable terminal
operations cockpit with a deliberately silly "PR Wrangler" streak: talking bull
moments, lasso references, Western-flavored loading states, and fun motion are
welcome as chrome around the workflow.

### Aesthetic Direction

The core UI is dense, keyboard-first, and built for repeated triage. Keep the
table work-focused, legible, and stable; use personality in headers, loading
states, empty states, overlays, and transitions rather than inside PR data
itself. Good references are agentic CLI tools and playful terminal apps such as
`agent-of-empires`: useful first, but willing to be memorable.

Use the existing Bubble Tea / Lip Gloss terminal visual system, semantic color
tokens, and supported dark terminal themes. Maintain the current accessibility
baseline: textual status and action labels, color never as the only cue,
contrast-aware palettes, `NO_COLOR` support, and decorative ASCII art kept away
from decision-critical data.

### Design Principles

- Optimize for fast PR triage before decoration: the table must stay scannable,
  stable, and keyboard-efficient.
- Let the brand be playful in the surrounding chrome, not in the data cells
  where maintainers make decisions.
- Make agentic actions feel immediate and trustworthy with clear states,
  progress, and outcomes.
- Preserve terminal robustness: responsive column math, ANSI-safe truncation,
  readable monochrome output, and no raw color escapes outside the style layer.
- Validate visual changes with the demo preview pipeline, extending mock data
  whenever a new visible state is introduced.
