# knuckle — Agent Context

> **Audience:** AI coding agents (Claude Code, pi, Copilot, Cursor). Humans want
> `README.md`. This file is the authoritative description of how to work in this
> repo without breaking it.
>
> **Bar:** match CNCF-incubating rigor. Every change must keep `just ci` green,
> respect the package boundaries, and preserve the safety invariants below.

---

## What This Repo Is

A modern TUI installer for [Flatcar Container Linux](https://www.flatcar.org/),
targeting bare-metal deployments. Built in Go on the charm.sh stack (Bubble Tea
v2, Lip Gloss v2, Huh v2). The wizard assembles an Ignition config and hands
off to `flatcar-install` — knuckle never writes partitions itself.

- **Module:** `github.com/projectbluefin/knuckle` (Go 1.26+)
- **License:** Apache-2.0
- **Status:** v0.2.1 released; install → reboot → SSH verified end-to-end in QEMU.
- **Distribution:** `knuckle` binary + installer ISO produced from
  `.github/workflows/release.yml` on `v*` tags.

## v1 Supported Scope

- **Architecture:** x86_64 + ARM64 (dual-arch ISOs since v0.4.0)
- **Storage:** single target disk (no RAID, LVM, LUKS)
- **Networking:** DHCP + simple static IPv4 only
- **UI Language:** English only
- **Sysexts:** official Flatcar Bakery entries only (via GitHub Releases API)
- **Config mode:** guided local generation OR external Ignition URL passthrough
  (`Ctrl+A`) — mutually exclusive, no merging

Anything outside this list belongs in an issue, not a PR.

---

## Build / Test / Lint

```bash
just              # list recipes
just ci           # tidy + gofmt + vet + lint + vuln + test-race + cover-check + build
just build        # GOOS=linux GOARCH=amd64 CGO_ENABLED=0 → bin/knuckle
just test         # go test ./...
just fmt          # gofmt -w .
just fmt-check    # CI gate: fails if any file is not gofmt-clean
just vuln         # govulncheck ./...  (auto-installs into $GOBIN)
just cover        # statement coverage profile → cover.out, prints total
just cover-check  # per-package coverage threshold gate
just headless-test       # build + run a canned JSON config (CI gate, runs on host)
just vm                  # install in QEMU VM → auto-boots installed system after
just vm-e2e              # automated 4-pass: DHCP · static · sysext · NVIDIA
just boot-iso            # build ISO → boot in QEMU GTK window (requires -cpu host; uses bin/knuckle)
just e2e                 # build ISO → boot in QEMU GTK window → interactive install
```

`just ci` is the pre-push gate. CI re-runs every step in
`.github/workflows/ci.yml` plus CodeQL / Scorecard / dependency-review in
`.github/workflows/security.yml`. If CI fails, fix it — never `--no-verify`.

## Safety Invariants (do not violate)

1. **Never run real `flatcar-install` on the host.** `just headless-test` runs
   on the host and only validates config generation — it does not call
   `flatcar-install`. Real installs run only inside QEMU (`just vm`, `just vm-e2e`).
2. **All system commands route through `internal/runner`.** No
   `exec.Command` outside `internal/runner`. Reboot is threaded via
   `rebootFn func(context.Context) error` injected from `cmd/knuckle/main.go`.
3. **Disk identity is `/dev/disk/by-id`.** Display model, serial, size,
   transport, removable flag. Never trust `/dev/sdX` enumeration order.
4. **Never log to stdout.** Bubble Tea owns it. Use `log/slog` with a file
   handler (default `/tmp/knuckle.log`, override via `--log-file`).
5. **Ignition contains secrets** (SSH keys, hashed passwords). Write it with
   `os.CreateTemp` (O_EXCL), `chmod 0600`, defer `os.Remove`. Pattern in
   `internal/install/install.go:WriteIgnitionFile`.

---

## Package Boundaries

| Package           | Responsibility                                                     | Coverage |
| ----------------- | ------------------------------------------------------------------ | -------- |
| `cmd/knuckle`     | CLI entrypoint, flag parsing, runner wiring                        | n/a      |
| `internal/model`  | Pure data types — `InstallConfig`, `DiskInfo`, `NetworkInterface`  | 93%      |
| `internal/runner` | `Runner` interface: `RealRunner`, `DryRunner`, `SpyRunner`         | 81%      |
| `internal/probe`  | `lsblk` + `ip addr` JSON parsing, `/dev/disk/by-id` resolution     | 82%      |
| `internal/validate` | Hostname, CIDR, gateway, SSH key, timezone, disk path validators | 95%      |
| `internal/bakery` | sysext catalog + Flatcar release/SBOM fetchers, SHA512 check       | 85%      |
| `internal/github` | SSH key fetch + GitHub Releases API client                         | 93%      |
| `internal/ignition` | Butane assembly + in-process Butane→Ignition compilation         | 92%      |
| `internal/install` | `flatcar-install` orchestration via runner                        | 79%      |
| `internal/iso`    | Installer ISO builder helpers                                      | 100%     |
| `internal/headless` | `--headless --config` JSON-driven install path                   | 88%      |
| `internal/wizard` | Step state machine, navigation, validation gates                   | 77%      |
| `internal/tui`    | Bubble Tea view models (one sub-model per step), forms             | 79%      |

Targets enforced by `just cover-check` are deliberately set ≤ current numbers
so the gate guards against *regression*. Long-term aspirations live in
`docs/CI-AND-TESTING.md` — raise the gate as coverage rises.

### Dependency Graph (acyclic; enforced by import structure)

```
model    ← leaf, zero internal imports — everything depends on it
runner   ← probe, install, headless (injected via interface)
validate ← tui (field-level), ignition (final check), headless
probe    ← wizard/tui (disk + network data)
bakery   ← wizard/tui (sysext catalog + channel info)
github   ← wizard (SSH key fetch)
ignition ← install, wizard
install  ← wizard, headless
headless ← cmd/knuckle
wizard   ← tui, cmd/knuckle
tui      ← cmd/knuckle
```

`go vet ./...` runs in CI; cycle violations break the build.

---

## Architecture Decisions

1. **Runner abstraction.** Every external command goes through
   `internal/runner.Runner`. Three implementations: `RealRunner` (prod),
   `DryRunner` (no-op + structured logging), `SpyRunner` (test recorder).
2. **Flatcar Butane variant.** `variant: flatcar` (not generic CoreOS),
   compiled in-process via `github.com/coreos/butane` v0.27+ →
   `ignition.CompileToIgnition()`. No `butane` CLI on the target system.
   Rationale: `docs/BUTANE-DEPENDENCY.md`.
3. **Mutually exclusive config modes.** Guided OR external Ignition URL
   (`Ctrl+A`). No merge logic.
4. **Disk identity via `/dev/disk/by-id`.** Falls back to raw device path only
   when `/dev/disk/by-id/` is absent (CI containers). See
   `internal/probe/probe.go:resolveByIDPath`.
5. **TUI ↔ logic separation.** `internal/tui` renders; `internal/wizard`
   transitions. No business logic in view models.
6. **Shared data model.** `internal/model` owns every cross-package type.
   Wizard builds them, TUI reads/writes fields, ignition consumes, validate
   checks.
7. **huh.Form for form steps.** Welcome, Network, User, Review use
   `charmbracelet/huh` with the Dracula theme. Storage, Sysext, Update,
   Install, Done are raw Bubble Tea. Validation via `.Validate()` callbacks.
8. **Supply-chain signals.** SBOM JSON (SPDX) is the primary version source.
   SHA512 against `.DIGESTS` is verified. GPG signature on the digest file is
   fully verified via `github.com/ProtonMail/go-crypto` against the embedded
   Flatcar signing key (`internal/bakery/keys/flatcar-signing.asc`).
9. **Headless mode mirrors the TUI.** `--headless --config <file.json>` drives
   `internal/headless` through the same `internal/install` path. New TUI
   fields must round-trip through the headless config schema.

---

## Test Pyramid

| Layer        | Where                                  | What                                                | Ghost? |
| ------------ | -------------------------------------- | --------------------------------------------------- | ------ |
| Unit         | `internal/**/_test.go`                 | Pure logic, fixture-driven                          | dev only |
| Golden       | `internal/ignition/testdata/`          | Butane → Ignition output diffs (`-update` rewrites) | dev only |
| Integration  | `//go:build integration` (not in CI)   | Real network: GitHub API, Flatcar release server    | dev only |
| Headless e2e | `just headless-test`                   | Build + canned JSON config, runs on host (CI gate)  | ✅ ghost |
| VM           | `just vm`                              | Install in QEMU, auto-boots installed system after  | local only (interactive TUI) |
| VM automated | `just vm-e2e`                          | 4-pass: DHCP, static network, sysext (docker), NVIDIA — fully automated | ✅ ghost |
| Ghost PR test | `scripts/qa-test-pr.sh <PR>`          | Per-PR: build + unit + VM dry-run + headless install, report to PR | ✅ ghost only |
| ISO e2e      | `just e2e`                             | Build ISO → boot in QEMU GTK window → interactive install | local only (requires display) |

CI runs unit + race + lint + vuln + coverage gate. `just vm-e2e` and `scripts/qa-test-pr.sh <PR>` run the automated test suite. `just e2e`/`just vm` require a local display.

### PR Verification

Load the **`knuckle-qa` skill** — it defines per-domain tiers, ghost testlab procedure, and the strike report policy.
Load the **`ghost-testlab` skill** — it defines the homelab topology and shared infrastructure.

---

## Working in this repo as an agent

### Claude Code

**Role in this repo:** Claude Code (Sonnet 4.6+) is the designated final
principal-engineer review agent. Before any release tag, run the PE checklist
below using Claude Code with the slm MCP wired — it has full cross-session
context of every prior review finding.

**Memory (slm) wiring — one-time setup:**

```bash
claude mcp add slm -s user -- podman exec -i systemd-superlocalmemory-slm slm mcp
# restart Claude Code — mcp__slm__* tools will surface in the session
```

**Verification:** ask Claude Code to `mcp__slm__get_status` — expect
`fact_count > 800`, `mode: "a"`. If absent or count is 0, memory is not wired.

When slm is wired, use `mcp__slm__recall(query=...)` — 4-channel retrieval
(semantic + spreading-activation + BM25 + temporal) vs FTS5-only search.
Bootstrap queries to run at session start:

```
recall("correction violation preference constraint workflow rule", limit=10)
recall("knuckle workflow patterns constraints project", limit=8)
recall("knuckle <task-description>", limit=5)
```

**If memory is absent:** say so and proceed. This repo does not block on it.
The full bootstrap from `~/src/AGENTS.md` is for the pi agent only.

**Tool pins:** golangci-lint and govulncheck versions are pinned in the
Justfile (`GOLANGCI_LINT_VERSION`) and go.mod (`tool` directive). When
bumping a tool version: update both places and commit go.mod + go.sum.

```bash
just tools          # install / verify pinned tool binaries
just ci             # full pre-push gate (now includes headless e2e)
```

### ISO build internals

The installer ISO modifies Flatcar's `usr.squashfs` directly — the only reliable
injection method for Flatcar PXE live boot.

- **Flatcar PXE initrd** = cpio with only `etc/` (empty) + `usr.squashfs`. No `/init`
  in the external cpio; dracut init is embedded in `vmlinuz`. Appended cpio overlays
  are abandoned at pivot_root — there is no `apply-live-updates.sh` hook.
- **`squashfs-root/` = `/usr/` in the live system.** So `squashfs-root/bin/knuckle` →
  `/usr/bin/knuckle`. Place units in `squashfs-root/lib/systemd/system/`.
- **Binary selection:** `scripts/build-iso.sh` uses `bin/knuckle` (built by `just build`
  with `CGO_ENABLED=0`). Never use the repo-root binary — it may contain AVX instructions
  that crash with `trap invalid opcode` in QEMU.
- **QEMU:** always pass `-cpu host`. Without it, AVX binaries silently crash. `just boot-iso`
  and `just e2e` both set this. Use `-display gtk` (not `-nographic`) to see the TUI on tty1.
- **Ignition in QEMU:** pass config via `-fw_cfg name=opt/org.flatcar-linux/config,file=config.ign`.
  The `ignition.config.data=` kernel cmdline parameter is silently ignored on the QEMU platform.
- **Build cache:** squashfs is content-addressed on `sha256sum bin/knuckle` — skips repack when
  binary unchanged.

### All agents

1. **Read this file, then the issue.** Don't infer scope from a commit subject.
2. **Declare SCOPE / GOAL / OUT OF SCOPE** before editing.
3. **One PR per issue.** Branch `feat/<slug>` or `fix/<slug>`. Conventional
   commits (`feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `ci:`, `chore:`).
4. **`just ci` is the gate.** If it fails, fix it; don't push.
5. **Push to `origin` (projectbluefin/knuckle) only.** No upstream pushes from
   automation.
6. **Touch `.github/workflows/*.yml`?** Coordinate via PR description — these
   are security-sensitive. CodeQL + Scorecard run on every push.
7. **Adding a new external command?** Wire it through `runner.Runner`. Period.
8. **Adding a new disk-touching code path?** Test it in QEMU via `just vm` or
   `just vm-e2e`. Unit tests use `SpyRunner` to assert commands without execution.

### Subagent dispatch

| Agent          | Use it for                                                      |
| -------------- | --------------------------------------------------------------- |
| `Explore`      | "where is X defined", broad code search                         |
| `Plan`         | Multi-file changes, architectural decisions                     |
| `QA`           | Edge-case enumeration, fixture gaps, test-pyramid review        |
| `Principal SE` | Pre-release architecture audit, blocker classification          |
| `Security`     | Any change touching disk writes, network, credentials, ignition |

Don't dispatch a subagent for single-file edits or single grep queries — do
those inline.

---

---

## PR Review + Ghost Test Workflow

Load the **`knuckle-qa` skill** before reviewing any PR.
Ghost homelab details live in the **`ghost-testlab` skill**.
Strike report format and posting rules live in the **`knuckle-qa` skill**.

All process policy belongs in skills — not in this file.

---

## Routine Maintenance

**Dependency bumps** — bump, run `just ci`, verify `just vm` still works:
```bash
go get -u ./...
go mod tidy
just ci
just vm
```

**Tool version bumps** — update `GOLANGCI_LINT_VERSION` in Justfile AND
`.github/workflows/ci.yml` together, then `just tools && just ci`.

**Flatcar release tracking** — bakery fetches current versions live; no manual
update needed. To force a check: `go test ./internal/bakery/... -run TestFetch`.

**Flatcar manual update on a running node:**
```bash
sudo update_engine_client -update   # trigger download
sudo update_engine_client -status   # watch progress
sudo systemctl reboot               # apply (if REBOOT_STRATEGY=off)
```

**Release tag checklist:**
1. `just tools && just ci` — must be green
2. `just vm` — manual install walkthrough, confirm SSH works on installed system
3. `just vm-e2e` — all 4 passes must exit 0 (DHCP · static · sysext · NVIDIA)
4. `git tag v0.X.Y && git push origin v0.X.Y` — triggers release.yml

---

## Reference

- [Flatcar Container Linux](https://www.flatcar.org/)
- [Flatcar Bakery (sysexts)](https://www.flatcar.org/docs/latest/provisioning/sysext/)
- [Butane / Ignition](https://coreos.github.io/butane/config-flatcar-v1_1/)
- [charm.sh](https://charm.sh) — Bubble Tea, Lip Gloss, Huh, Bubbles
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/)
- [OSSF Scorecard](https://github.com/ossf/scorecard) — runs weekly in `security.yml`
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) — runs every PR
