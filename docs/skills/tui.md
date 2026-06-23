---
name: tui
description: "TUI development for knuckle using charmbracelet/bubbletea, huh, and lipgloss. Use when building or modifying the installer wizard, form flows, or terminal UI components."
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
resolve-library-id "/charmbracelet/bubbletea"  → get-library-docs
resolve-library-id "/charmbracelet/huh"         → get-library-docs
```

Do not guess API signatures. The charm.sh libraries evolve quickly — training data is stale.

## Architecture

Knuckle follows the bubbletea Model/Update/View pattern:

- `model` — all state
- `Update(msg)` — pure state transitions; return `(Model, tea.Cmd)`
- `View()` — renders from current model state

Multi-step wizard = `huh.Form` with `huh.Group` per step.
Group transitions are internal to huh; do not re-implement them.

## Key patterns

See `docs/TUI-WIZARD-PATTERNS.md` for research notes and concrete examples derived from reading huh source.

## What NOT to do

- Do not hand-roll form navigation — huh handles group transitions
- Do not call `lipgloss.Style` methods without checking current API in Context7 (method names changed between releases)
- Do not assume bubbletea message types — read the source via Context7
