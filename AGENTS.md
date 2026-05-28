# knuckle — Agent Context

> **Audience:** AI coding agents (Claude Code, pi, Copilot, Cursor). Humans want `README.md`.
>
> **Bar:** CNCF-incubating rigor. Every change keeps `just ci` green, respects package
> boundaries, and preserves the safety invariants below.

---

## What This Repo Is

A TUI installer for [Flatcar Container Linux](https://www.flatcar.org/), targeting bare-metal.
Built in Go on charm.sh (Bubble Tea v2, Lip Gloss v2, Huh v2). Assembles an Ignition config
and hands off to `flatcar-install` — knuckle never writes partitions itself.

- **Module:** `github.com/projectbluefin/knuckle` (Go 1.26+)
- **License:** Apache-2.0
- **Status:** v0.7.0; full install → reboot → SSH verified end-to-end in QEMU; dual-arch ISOs (amd64 + arm64); cosign keyless signing on releases.

## v1 Supported Scope

- Architecture: x86_64 + ARM64
- Storage: single target disk (no RAID, LVM, LUKS)
- Networking: DHCP + simple static IPv4 only
- Sysexts: official Flatcar Bakery entries only (via GitHub Releases API)
- Config: guided TUI OR external Ignition URL passthrough (`Ctrl+A`) — mutually exclusive

Anything outside this list belongs in an issue, not a PR.

---

## Build / Test / Lint

```bash
just              # list all recipes
just ci           # tidy + gofmt + vet + lint + vuln + test-race + cover-check + build
just build        # GOOS=linux GOARCH=amd64 CGO_ENABLED=0 → bin/knuckle
just test         # go test ./...
just cover-check  # per-package coverage threshold gate
just headless-test       # config-gen e2e (CI gate, runs on host)
just vm                  # install in QEMU → auto-boots installed system after
just vm-e2e              # automated 4-pass: DHCP · static · sysext · NVIDIA
just boot-iso            # build ISO → boot in QEMU GTK window (requires -cpu host)
just e2e                 # build ISO → boot in QEMU GTK window → interactive install
```

`just ci` is the pre-push gate. Never `--no-verify`.

---

## Safety Invariants ⛔

1. **Never run `flatcar-install` on the host.** `just headless-test` only validates config generation. Real installs run only inside QEMU.
2. **All system commands route through `internal/runner`.** No `exec.Command` outside that package. Reboot wired via `rebootFn func(context.Context) error` injected from `cmd/knuckle/main.go`.
3. **Disk identity is `/dev/disk/by-id`.** Never trust `/dev/sdX` enumeration order.
4. **Never log to stdout.** Bubble Tea owns it. Use `log/slog` with a file handler (`/tmp/knuckle.log` default).
5. **Ignition contains secrets.** Write with `os.CreateTemp` (O_EXCL), `chmod 0600`, `defer os.Remove`. See `internal/install/install.go:WriteIgnitionFile`.

---

## Package Boundaries

Coverage gates are authoritative in [`docs/CI-AND-TESTING.md`](docs/CI-AND-TESTING.md#coverage-gate).

| Package             | Responsibility                                                    |
| ------------------- | ----------------------------------------------------------------- |
| `cmd/knuckle`       | CLI entrypoint, flag parsing, runner wiring                       |
| `internal/model`    | Pure data types — `InstallConfig`, `DiskInfo`, `NetworkInterface` |
| `internal/runner`   | `Runner` interface: `RealRunner`, `DryRunner`, `SpyRunner`        |
| `internal/probe`    | `lsblk` + `ip addr` JSON parsing, `/dev/disk/by-id` resolution   |
| `internal/validate` | Hostname, CIDR, gateway, SSH key, timezone, disk path validators  |
| `internal/bakery`   | sysext catalog + Flatcar release/SBOM fetchers, SHA512 + GPG check|
| `internal/github`   | SSH key fetch + GitHub Releases API client                        |
| `internal/ignition` | Butane assembly + in-process Butane→Ignition compilation          |
| `internal/install`  | `flatcar-install` orchestration via runner                        |
| `internal/iso`      | Installer ISO builder helpers                                     |
| `internal/headless` | `--headless --config` JSON-driven install path                    |
| `internal/wizard`   | Step state machine, navigation, validation gates                  |
| `internal/tui`      | Bubble Tea view models (one sub-model per step), forms            |

### Dependency Graph (acyclic — `go vet` enforced)

```
model    ← leaf, zero internal imports
runner   ← probe, install, headless (injected via interface)
validate ← tui, ignition, headless
probe    ← wizard/tui
bakery   ← wizard/tui
github   ← wizard
ignition ← install, wizard
install  ← wizard, headless
headless ← cmd/knuckle
wizard   ← tui, cmd/knuckle
tui      ← cmd/knuckle
```

---

## Architecture Decisions

1. **Runner abstraction.** Every external command through `internal/runner.Runner` — `RealRunner` (prod), `DryRunner` (no-op), `SpyRunner` (test recorder).
2. **Flatcar Butane variant.** `variant: flatcar` compiled in-process via `ignition.CompileToIgnition()`. No `butane` CLI on target. See `docs/BUTANE-DEPENDENCY.md`.
3. **Mutually exclusive config modes.** Guided TUI OR external Ignition URL. No merge logic.
4. **Disk identity via `/dev/disk/by-id`.** Falls back to raw device path only when `/dev/disk/by-id/` absent (CI containers). See `internal/probe/probe.go:resolveByIDPath`.
5. **TUI ↔ logic separation.** `internal/tui` renders; `internal/wizard` transitions. No business logic in view models.
6. **Shared data model.** `internal/model` owns every cross-package type.
7. **huh.Form for form steps.** Welcome, Network, User, Review use `charmbracelet/huh` + Dracula theme. Storage, Sysext, Update, Install, Done are raw Bubble Tea.
8. **Supply-chain.** SBOM JSON (SPDX) is the primary version source. SHA512 + GPG against `.DIGESTS.asc` (PGP clearsigned — verify with `gpg --decrypt`, not `gpg --verify`).
9. **Headless mirrors the TUI.** `--headless --config <file.json>` drives the same `internal/install` path. New TUI fields must round-trip through the headless config schema.

---

## Agent Rules

1. **Read this file, then the issue.** Don't infer scope from a commit subject.
2. **Declare SCOPE / GOAL / OUT OF SCOPE** before editing.
3. **One PR per issue.** Branch `feat/<slug>` or `fix/<slug>`. Conventional commits (`feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `ci:`, `chore:`).
4. **`just ci` is the gate.** If it fails, fix it; don't push.
5. **Push to `origin` (projectbluefin/knuckle) only.** No upstream pushes from automation.
6. **Workflow files (`.github/workflows/*.yml`):** security-sensitive, cannot be auto-merged. Coordinate via PR description.
7. **New external command?** Wire through `runner.Runner`. Period.
8. **New disk-touching code?** Test in QEMU via `just vm` or `just vm-e2e`. Unit tests use `SpyRunner`.

---

## Docs — Load on Demand

These docs live in `docs/` and improve continuously. Load the relevant one for your task — don't load all of them.

| Task | Document |
| ---- | -------- |
| Coverage gates, test pyramid, CI pipeline, ISO build internals | [`docs/CI-AND-TESTING.md`](docs/CI-AND-TESTING.md) |
| PR test matrix, tier classification, domain assertions, ghost QA evidence | [`docs/PR-TEST-MATRIX.md`](docs/PR-TEST-MATRIX.md) |
| Ghost lab setup, port inventory, KubeVirt, QA workflow, conflict resolution | [`docs/GHOST-LAB.md`](docs/GHOST-LAB.md) |
| Release tag checklist, VM verification, blocker history | [`docs/RELEASE.md`](docs/RELEASE.md) |
| Headless config schema, field reference, validation rules | [`docs/HEADLESS-CONFIG.md`](docs/HEADLESS-CONFIG.md) |
| Security posture, threat model, disclosure path | [`docs/SECURITY.md`](docs/SECURITY.md) |
| Sysext catalog, Bakery support tiers, extension behavior | [`docs/SYSEXTS.md`](docs/SYSEXTS.md) |
| Troubleshooting runbook, first-boot diagnostics | [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md) |
| Butane-as-library rationale | [`docs/BUTANE-DEPENDENCY.md`](docs/BUTANE-DEPENDENCY.md) |

---

## Routine Maintenance

```bash
# Dependency bumps
go get -u ./... && go mod tidy && just ci && just vm

# Tool version bumps — update GOLANGCI_LINT_VERSION in Justfile AND .github/workflows/ci.yml together
just tools && just ci

# Flatcar release tracking — fetched live by bakery; to force a check:
go test ./internal/bakery/... -run TestFetch
```

---

## Reference

[Flatcar](https://www.flatcar.org/) · [Bakery](https://www.flatcar.org/docs/latest/provisioning/sysext/) · [Butane](https://coreos.github.io/butane/config-flatcar-v1_1/) · [charm.sh](https://charm.sh) · [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/) · [OSSF Scorecard](https://github.com/ossf/scorecard) · [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
