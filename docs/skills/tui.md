---
name: tui
description: "TUI development for knuckle using charm.sh libraries (bubbletea, huh, lipgloss, bubbles). Use when building or modifying the installer wizard, form flows, or terminal UI components."
metadata:
  type: reference
  context7-sources:
    - /charmbracelet/bubbletea
    - /charmbracelet/huh
    - /charmbracelet/lipgloss
    - /charmbracelet/bubbles
---

# TUI — knuckle

## When to Use

- Building or modifying installer wizard steps
- Debugging form transitions, model updates, or rendering
- Working with any charm.sh library

## Mandatory first step

Before touching any bubbletea/huh/lipgloss API, run Context7:

```
resolve-library-id "/charmbracelet/bubbletea"  → get-library-docs (v2.0.0)
resolve-library-id "/charmbracelet/huh"         → get-library-docs (v2.0.0)
resolve-library-id "/charmbracelet/bubbles"     → get-library-docs (v2.0.0)
resolve-library-id "/charmbracelet/lipgloss"    → get-library-docs (v2.0.0)
```

Do not guess API signatures. The charm.sh libraries evolve quickly.

> **Import paths changed.** Go modules now live at `charm.land`, not `github.com/charmbracelet`.
> Always use the `charm.land` import paths in Go source:
> ```go
> import (
>     tea  "charm.land/bubbletea/v2"
>     "charm.land/huh/v2"
>     "charm.land/lipgloss/v2"
>     "charm.land/bubbles/v2/list"
> )
> ```
> The Context7 library IDs still use `/charmbracelet/` — that is the repo name and is correct for lookups.

## Architecture

Knuckle follows the bubbletea Model/Update/View pattern:

- `model` — all state
- `Update(msg)` — pure state transitions; return `(Model, tea.Cmd)`
- `View()` — renders from current model state

Multi-step wizard = `huh.Form` with `huh.Group` per step.

## What NOT to do

- Do not hand-roll form navigation — huh handles group transitions
- Do not call `lipgloss.Style` methods without checking current API in Context7
- Do not assume bubbletea message types — read the source via Context7
- Do not use `github.com/charmbracelet/` import paths — they are the old module location

## Hardware serial consoles

Bubble Tea sizes its renderer from the output TTY. A physical serial console
can report `0x0` rows and columns because serial links do not negotiate PTY
geometry. Before starting Bubble Tea, the CLI should check the output size and
the active console, then repair only an active serial console with a fixed
geometry (`stty -F /dev/console rows 40 cols 80`). Forty rows leave enough
vertical space for the initial installer view while retaining a conventional
80-column serial width. Route that command through
`internal/runner`; do not invoke `exec.Command` from the TUI package. Non-serial
consoles and already-sized terminals must remain untouched.

## Research notes

See `docs/TUI-WIZARD-PATTERNS.md` for concrete examples derived from reading huh source.

## Accessibility pattern: destructive review confirms

- Keep a dedicated `StepReview` theme instead of changing all forms globally.
- Use `huh.ThemeFunc(func(isDark bool) *huh.Styles { ... })` and start from `huh.ThemeDracula(isDark)`, then override focused title/button contrast for the destructive confirm.
- Avoid fixed review-dialog widths; use a bounded width derived from `m.width` so summary text wraps cleanly on both narrow and wide terminals.
- In review summaries, prioritize scan order: explicit wipe warning first, then target disk, then install plan details.
