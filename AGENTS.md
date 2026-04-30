# knuckle — Agent Context

## What This Repo Is

A modern TUI installer for Flatcar Container Linux, targeting bare-metal deployments.
Built with Go and the charm.sh ecosystem (Bubble Tea, Lip Gloss, Huh).

**Status:** Early development — scaffolding phase.

## v1 Supported Scope

- **Architecture:** x86_64 only (ARM64 is future work)
- **Storage:** Single target disk (no RAID, LVM, or LUKS)
- **Networking:** DHCP + simple static IPv4 only
- **UI Language:** English only (no translated strings)
- **Sysexts:** Official Flatcar Bakery entries only
- **Config mode:** Guided local generation OR external Ignition URL — mutually exclusive

## Build / Test / Lint

```bash
just ci        # full pipeline: tidy + lint + test-race + build
just build     # compile binary to bin/knuckle
just test      # go test ./...
just test-race # go test -race ./...
just lint      # golangci-lint run
just fmt       # gofumpt
just vuln      # govulncheck
just run       # go run ./cmd/knuckle
```

## Safety Rules

- **Never run real `flatcar-install` on host.** Use `--dry-run` or QEMU/loopback for testing.
- All system commands (lsblk, ip, flatcar-install) go through `internal/runner` — never `exec.Command` directly from TUI code.
- Disk selection must use `/dev/disk/by-id` where possible; display model, serial, size, transport, removable flag.
- **Never log to stdout** — Bubble Tea owns stdout. Use `log/slog` with a file handler.

## Package Boundaries

| Package | Responsibility |
|---|---|
| `cmd/knuckle` | CLI entrypoint, flag parsing (`--dry-run`, `--log-file`, `--channel`) |
| `internal/model` | Pure data types (InstallConfig, DiskSelection, NetworkConfig, etc.) — zero deps |
| `internal/wizard` | Step flow state machine, navigation logic, validation gates |
| `internal/tui` | Bubble Tea view models, rendering (one sub-model per step) |
| `internal/probe` | System probing (disks via lsblk, network via ip link, hardware) |
| `internal/runner` | exec.Command wrapper, `--dry-run` support, output capture, test spy |
| `internal/bakery` | HTTP client for Flatcar Bakery sysext catalog |
| `internal/ignition` | Butane config assembly (Flatcar variant), Ignition compilation |
| `internal/install` | flatcar-install orchestration via runner |
| `internal/validate` | Input validation, config consistency checks |

## Dependency Graph (no cycles allowed)

```
model ← (leaf, zero imports — everyone depends on it)
runner ← probe, install (injected via interface)
validate ← tui (field-level), ignition (final check)
probe ← wizard/tui (provides disk/network data)
bakery ← wizard/tui (provides sysext catalog)
ignition ← install, wizard
install ← wizard
wizard ← tui, cmd/knuckle
tui ← cmd/knuckle
```

## Architecture Decisions

1. **Runner abstraction** — All external commands go through `internal/runner`. This enables dry-run mode, test fixtures, and safe CI.
2. **Flatcar Butane variant** — Use `variant: flatcar` (not generic CoreOS) when generating Butane configs. Import via `github.com/coreos/butane/config`.
3. **Mutually exclusive config modes** — v1 supports either guided local generation OR external Ignition URL passthrough. No merge logic.
4. **Disk identity** — Use `/dev/disk/by-id` paths. Never rely on `/dev/sda` ordering.
5. **TUI ↔ logic separation** — `internal/tui` renders views; `internal/wizard` manages state transitions. No business logic in view models.
6. **Shared data model** — `internal/model` owns all data types. Wizard builds them, TUI reads/writes fields, ignition consumes them, validate checks them.
7. **One top-level Bubble Tea Model** — Parent model dispatches to step sub-models. Step transitions are `tea.Cmd`s. Use `huh.Form` for form steps, raw Bubble Tea for disk table and progress.

## Testing Strategy

- Unit tests with fixture data in `testdata/`
- Table-driven tests for `validate`, `probe`, `bakery`
- Golden file tests for `ignition` (with `-update` flag)
- Runner abstraction allows testing install/probe logic without real hardware
- Integration tests gated behind `//go:build integration`
- No real disk writes in CI — `--dry-run` is default in test mode
- Coverage targets: ≥80% for validate/ignition/probe/runner, ≥70% for bakery/install, ≥60% for wizard

## Agent Workflow — Required Skills

When an agent works on this repo, load these skills in order:

### Always Load (every session)
```
cat ~/src/skills/workflow/SKILL.md          # session lifecycle, scope declaration
cat ~/src/skills/github-issues/SKILL.md     # issue triage, labels, closure protocol
```

### Load By Task Type

| Task | Skills to Load |
|---|---|
| Implementing a feature issue | `workflow` + `github-issues` |
| Writing or updating tests | `workflow` + TDD skills (`tdd-red`, `tdd-green`, `tdd-refactor`) |
| CI/CD changes (`.github/workflows/`) | `workflow` + `github-actions-expert` |
| Multi-file architecture work | `workflow` + `blueprint-mode` + `subagent-discipline` |
| Code review | `workflow` + `receiving-code-review` or `requesting-code-review` |
| Debugging a failing test | `workflow` + `systematic-debugging` |
| Release / binary distribution | `workflow` + `git-pr-workflow` |
| Security review (disk/network handling) | `workflow` + `se-security-reviewer` |

### Agent Dispatch Patterns

| Agent Type | When to Use |
|---|---|
| **SWE** (`swe-subagent`) | Implementing a single issue (feature, bugfix) |
| **TDD Red** | Writing failing tests for a new feature before implementation |
| **TDD Green** | Making tests pass with minimal code |
| **TDD Refactor** | Cleaning up after green phase |
| **QA** | Test plan review, edge case analysis, bug hunting |
| **Principal SE** | Architecture decisions, package boundary questions |
| **Rubber Duck** | Plan critique before implementation, blind spot detection |
| **Security Reviewer** | Any code touching disk writes, network config, or credential handling |
| **GitHub Actions Expert** | CI workflow authoring, ISO builder pipeline |

### Implementation Workflow (per issue)

```
1. Load workflow + relevant domain skill
2. Read the issue body + any enrichment comments
3. Create feature branch: feat/<issue-slug>
4. Implement with TDD: red → green → refactor
5. Run `just ci` — must pass
6. Commit with conventional commit: feat|fix|refactor|test: <description>
7. Push to origin, report compare URL
8. Close issue with evidence (command + output)
```

### Conventional Commit Types

```
feat:     New feature or capability
fix:      Bug fix
test:     Adding or updating tests
refactor: Code restructuring (no behavior change)
docs:     Documentation updates
ci:       CI/CD workflow changes
chore:    Maintenance (deps, tooling)
```

### Key Rules for This Repo

1. **Issue-first** — Every PR must reference an issue number
2. **Branch-per-feature** — One branch per issue, named `feat/<slug>` or `fix/<slug>`
3. **`just ci` gate** — Must pass before any push
4. **No real installs** — `--dry-run` in all tests and local dev
5. **Golden files** — Run `go test ./internal/ignition -update` when Ignition output intentionally changes
6. **Fixture-driven** — Probe tests use committed JSON fixtures in `testdata/`, never live system calls
7. **Interfaces for injection** — Runner, Prober, BakeryClient, Installer all defined as interfaces
8. **Co-authored-by trailer** — All agent commits include `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`

## Reference

- [Flatcar Container Linux](https://www.flatcar.org/)
- [Flatcar Bakery (sysexts)](https://www.flatcar.org/docs/latest/provisioning/sysext/)
- [Butane / Ignition](https://coreos.github.io/butane/config-flatcar-v1_1/)
- [charm.sh ecosystem](https://charm.sh)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Huh (forms)](https://github.com/charmbracelet/huh)
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/)
