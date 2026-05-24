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

CI runs unit + race + lint + vuln + coverage gate. `just vm-e2e` and `scripts/qa-test-pr.sh` run on **ghost** (192.168.1.102). `just e2e`/`just vm` require a local display.

### Ghost Testlab

Ghost (192.168.1.102) is the dedicated headless QEMU host for VM-level testing:
- Flatcar 4593.2.1 base image at `/var/tmp/knuckle-test/flatcar_base.img`
- 32 KVM cores, 46 GB RAM available, 205 GB NVMe
- Port range 2300–2315 reserved for PR test VMs
- **`hostfwd` binds `127.0.0.1`** — all VM SSH must run FROM ghost, not through it
- Full procedure: load `knuckle-qa` skill

### Per-PR Verification Policy

| PR labels | Minimum required before merge | Evidence standard |
|---|---|---|
| `domain:ci`, `kind/test`, docs only | `just ci` green | Unit test output |
| `domain:validate`, `domain:model`, `domain:runner` | `just ci` green | Unit test output |
| `domain:probe`, `domain:tui` | ghost Tier 1 (tool check + dry-run) | Tool versions + dry-run log |
| `domain:security` | ghost Tier 1 + bad-input rejection tests | Every invalid input must be **rejected** with quoted output |
| `domain:ignition`, `domain:headless` | **ghost Tier 3 (install + boot + assertions)** | Ignition files, hostname, authorized_keys from **booted installed system** |
| `domain:install` | **ghost Tier 3** | GPT table, lsblk, disk-by-id from **booted installed system** |
| swap feature | **ghost Tier 3** | `swapon --show`, `ls -lah /var/swapfile`, `free -h` from **booted system** |
| tailscale feature | **ghost Tier 3** | `stat -c "%a %n" /etc/tailscale/tailscale.env` (must be 600) from **booted system** |
| `domain:iso` | ghost Tier 3 + `hardware-repro` | GPT layout, ISO boot log |
| `size:XL` or >4 domains | Human `just vm-e2e` sign-off | All 4 passes green |

**The bar:** Tier 3 means the installed system booted and responded to SSH. Evidence is quoted command output from inside that system — not build logs, not unit test output, not exit codes alone.

> ⛔ Never queue a PR without a passing ghost test report. Code review approval alone is not sufficient.

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

## PR Review + Ghost Test Workflow

This is the canonical procedure for reviewing any knuckle PR. Follow it in
order, every time. No steps are optional.

### Step 0 — Session start (2 minutes)

```bash
# Check open PRs
gh pr list --repo projectbluefin/knuckle --state open

# Check ghost is reachable
ssh -o ConnectTimeout=5 jorge@192.168.1.102 "hostname && df -h /var/tmp | tail -1" 2>&1
```

If ghost is unreachable: do code review only (Step 2). Skip VM tests. Say so
explicitly in the review comment.

---

### Step 1 — Complexity gate (1 minute)

Fetch the PR metadata:

```bash
PR=<number>
gh pr view $PR --repo projectbluefin/knuckle --json title,labels,additions,deletions
```

**Skip to Step 2 only (no VM tests, no auto-merge) if ANY of:**

| Signal | Value that triggers skip |
|---|---|
| `size:XL` label | present |
| `domain:*` label count | > 4 |
| Lines changed | > 500 |
| Files touching `.github/workflows/` | any |
| Closes issues count | > 5 |

For complex PRs: write the code review, add this comment:
> "This PR is too large for automated ghost VM verification. A human `just vm-e2e` run on ghost (192.168.1.102) is required before merge."

---

### Step 2 — Code review (5–15 minutes)

**Read the full diff:**
```bash
gh pr diff $PR --repo projectbluefin/knuckle
```

**Check against safety invariants** (non-negotiable):
- [ ] No `exec.Command` outside `internal/runner`?
- [ ] Disk writes only inside QEMU (never on host)?
- [ ] Ignition tempfile uses `os.CreateTemp` + `chmod 0600` + `defer Remove`?
- [ ] No disk secrets in `slog` output?
- [ ] New external commands wired through `runner.Runner`?

**Check domain-specific patterns:**

| Domain | Key check |
|---|---|
| `install` | `wipefs → flatcar-install → sfdisk` order; DryRunner no-ops all three |
| `ignition` | Template `{{- end}}` balanced; `yamlEscape` on every user string |
| `headless` | `Validate()` called before `ToInstallConfig()`; SSH keys validated |
| `tui` | No business logic in view model; `wizard.Apply*` for mutations |
| `validate` | Table-driven tests; error messages include the bad value |
| `wizard` | Conditional steps check selector (e.g. `isTailscaleSelected()`) in Next/Previous/GoToStep |
| `bakery` | SHA512 + GPG both checked; no per-call `http.Client` creation |
| `iso` | `systemd.gpt_auto=0` on both boot entries; `--efi-boot-part` not `--isohybrid-gpt-basdat` |

**Rubber duck pass — mandatory before submitting:**
> Read your review back. For every "LGTM" ask: is this verified from the diff, or assumed?
> For every warning ask: would you accept this explanation if you were the author?

**Never post a review to GitHub until Step 3 (ghost test) passes.**
The review and test report are posted together as one comment in Step 5.

**Submit the review** via `gh pr review` or MCP tools. Use `APPROVE`,
`REQUEST_CHANGES`, or `COMMENT`. Do not open a new PR for fixes — push to
the existing branch.

---

### Step 3 — Ghost VM test (5–25 minutes depending on tier)

Run from the **dev machine** (not ghost — the script SCPs the binary to ghost):

```bash
cd ~/src/knuckle
./scripts/qa-test-pr.sh $PR 2>&1 | tee /tmp/knuckle-qa-pr-${PR}-report.md
echo "Exit: $?"
```

**What the script does automatically:**
1. Fetches PR labels and branch from GitHub
2. Complexity gate (exits 2 if too complex — you already checked in Step 1)
3. Checks out the PR head, runs `just ci` locally (all unit tests, lint, coverage)
4. Builds `bin/knuckle` from the PR head commit
5. SCPs the binary to ghost (`jorge@192.168.1.102:/var/tmp/knuckle-qa-pr-${PR}/`)
6. On ghost: allocates a free port (2300–2315), boots a fresh Flatcar VM (qcow2 CoW overlay on `flatcar_base.img`)
7. Waits for SSH (20 × 2s, fails fast with serial log if timeout)
8. Runs tier-appropriate tests (see below)
9. Writes a markdown report to stdout

**Tier selection by label:**

| Labels present | Tier | What runs | Evidence in report | Time |
|---|---|---|---|---|
| `domain:ci`, `kind/test`, `domain:validate`, docs | 0 | `just ci` only | Unit test output | ~2m |
| `domain:probe`, `domain:tui` | 1 | Tier 0 + VM tool check + `--dry-run` | Tool versions, dry-run log | ~5m |
| `domain:security` | 1+sec | Tier 1 + bad-input rejection tests | Each invalid input must be **rejected** (quoted output) | ~7m |
| `domain:install`, `domain:headless`, `domain:ignition`, swap, tailscale | **3** | Tier 1 + **full install + BOOT installed system** + domain assertions | GPT table, swapfile, env file modes, Ignition-provisioned files — **all quoted from installed system** | ~20m |
| `domain:iso` | 3 | Tier 3 + `hardware-repro` | ISO boot + GPT + install log | ~30m |

**Full QA science standard — Tier 3 reports must include:**
- Quoted `sfdisk -l` output (GPT intact)
- Quoted `swapon --show` (if swap PR)
- Quoted `stat -c "%a %n" /etc/tailscale/tailscale.env` (if tailscale PR)
- Quoted `ls ~/.ssh/authorized_keys` (Ignition applied)
- Quoted `hostname` (config applied correctly)
- Any `FAIL:` line causes the report to fail

Tier 3 is triggered automatically by the script based on labels. Never accept a
Tier 1/2 report for a PR that touches the installed system.

**If the script exits non-zero:** read the report. Common failures:
- `VM BOOT TIMEOUT` — check ghost port occupancy: `ssh jorge@ghost "ss -tlnp | grep 23"`
- `BUILD FAILED` — the PR has a compilation error; request changes
- `ASSERTIONS_FAILED` — the installed system did not match the expected state
- `BAD_PW_ACCEPTED_FAIL` — plaintext password was accepted (security regression)
- `INSTALL_FAILED` — `flatcar-install` exited non-zero; check install log in report

---

### Step 4 — Aggressive QA agent review

Dispatch the QA subagent with this exact brief:

```
You are an adversarial QA reviewer. Your job is to find what's wrong.
Be blunt — no softening. The PR is: <TITLE>

CODE DIFF:
<paste output of: gh pr diff $PR --repo projectbluefin/knuckle>

GHOST TEST REPORT:
<paste contents of /tmp/knuckle-qa-pr-${PR}-report.md>

Find:
1. Edge cases not covered by the test suite
2. Security issues (disk writes, credentials, path traversal)
3. Behavioral gaps between TUI and headless paths
4. Missing error paths in the new code
5. Test assertions that verify presence but not behavior

For each finding: BLOCKER / SHOULD-FIX / NIT, file:line, and exact fix.
If you find nothing significant: say so explicitly with your reasoning.
```

Capture output: `tee /tmp/knuckle-qa-pr-${PR}-qa-findings.md`

---

### Step 5 — Publish report and decide

**ONE COMMENT RULE: The strike report is the only comment posted on the PR. No other text.**

```bash
# 1. Post the full strike report as the single PR comment
gh pr comment $PR --repo projectbluefin/knuckle \
  --body-file /tmp/qa-stdout-${PR}.txt

# 2. Approve or request changes with NO body text (-b flag is forbidden here)
gh pr review $PR --repo projectbluefin/knuckle --approve          # if passing
gh pr review $PR --repo projectbluefin/knuckle --request-changes  # if failing
```

> ⛔ Never add a `-b` body to `gh pr review`. Never post a second comment. The report IS the review.

**Decision matrix:**

| Code review | Ghost tests | Action |
|---|---|---|
| APPROVE | PASS | Post report → `gh pr review --approve` (no body) → queue |
| APPROVE | FAIL | Post report → `gh pr review --request-changes` (no body) |
| REQUEST_CHANGES | any | Post report → `gh pr review --request-changes` (no body) |
| Complex (skipped) | skipped | Post Tier 0 CI result only → leave review |

**Queueing:**
```bash
# Before queuing: check the PR is not DIRTY
STATE=$(gh pr view $PR --repo projectbluefin/knuckle --json mergeStateStatus --jq .mergeStateStatus)
[[ "$STATE" == "DIRTY" ]] && {
  # Try GitHub rebase first
  gh pr update-branch $PR --repo projectbluefin/knuckle || {
    echo "Manual rebase required — see Step 6"
    exit 1
  }
}
gh pr merge $PR --repo projectbluefin/knuckle
```

---

### Step 6 — Conflict resolution (if PR is DIRTY)

Never open a new PR for a rebase. Push to the existing branch.
Check for file overlap before resolving — if the same file appears in multiple
open PRs, review them sequentially (queue one, wait for merge, then review next).

```bash
# 0. Check if GitHub can do it automatically
gh pr update-branch $PR --repo projectbluefin/knuckle && exit 0
```

Then proceed with local rebase only if the above fails.

```bash
# 1. Fetch the PR head
git fetch upstream pull/${PR}/head:pr${PR}-head

# 2. Create a rebase branch from current main
git checkout -b fix/pr${PR}-rebased upstream/main
git cherry-pick pr${PR}-head   # or git merge --no-commit pr${PR}-head

# 3. Resolve conflicts (keep both sides for additive changes)
git add <resolved files> && git cherry-pick --continue

# 4. Verify
go build ./...   # MUST exit 0 before git add
just ci          # must be green

# 5. Push to the EXISTING PR branch (maintainerCanModify: true required)
git remote add pr-author git@github.com:<AUTHOR>/knuckle.git 2>/dev/null || true
git fetch upstream main && git rebase upstream/main
git push pr-author fix/pr${PR}-rebased:<ORIGINAL_BRANCH> --force-with-lease

# 6. Queue (CI will re-run on the new head)
gh pr merge $PR --repo projectbluefin/knuckle
```

**If maintainerCanModify is false:** request author to rebase, comment with
the exact conflict resolution (paste the diff).

---

### Quick reference card

```
1. ghost reachable?    ssh jorge@192.168.1.102 hostname
2. PR complexity?      gh pr view $PR --repo projectbluefin/knuckle --json labels
3. Code review         gh pr diff $PR --repo projectbluefin/knuckle
4. Ghost test          ./scripts/qa-test-pr.sh $PR
5. QA agent            dispatch qa subagent with diff + report
6. Publish + queue     gh pr comment ... && gh pr merge $PR ...
```

**Ghost test port cleanup** (if ports are stuck):
```bash
ssh jorge@192.168.1.102 "
  echo '=== QEMU VMs ==='
  pgrep -a qemu-system | grep -v 'qemu.pid' || echo none
  echo '=== Ports 2300-2315 ==='
  ss -tlnp | grep ':23[0-1][0-9]' || echo none
  echo '=== /var/tmp knuckle dirs ==='
  ls -d /var/tmp/knuckle-qa-pr-* 2>/dev/null || echo none
"
# Kill a stuck VM:
ssh jorge@192.168.1.102 "kill \$(cat /var/tmp/knuckle-qa-pr-${PR}/qemu.pid 2>/dev/null) 2>/dev/null || true"
```

---


Run this before tagging any release. Anything red blocks the tag.

```bash
# One command to run them all — must be green on a clean checkout
just tools && just ci
```

Individual gates (all exercised by `just ci`):

- [ ] `go mod tidy && git diff --exit-code go.mod go.sum` — module graph clean
- [ ] `gofmt -l .` empty
- [ ] `go vet ./...` clean
- [ ] `.tools/golangci-lint run ./...` clean
- [ ] `go tool govulncheck ./...` — `No vulnerabilities found.`
- [ ] `go test -race ./...` — all 12 packages green
- [ ] `just cover-check` — all packages above gate thresholds
- [ ] `just headless-test` — config generation e2e passes (runs on host)
- [ ] `just vm-e2e` — all 4 passes green (DHCP, static, sysext, NVIDIA)
- [ ] `just build` — binary compiles
- [ ] `git status` clean — no untracked files in repo
- [ ] `grep -rn 'exec\.Command' --include='*.go' --exclude-dir=internal/runner .`
      → zero results (all reboot paths use `rebootFn` injected via runner)
- [ ] All claims in `README.md` still true
- [ ] `docs/REVIEW-*.md` reconciled — every blocker fixed or deferred with issue

**Blockers status:** B1 (GPG) ✓, B2 (reboot runner) ✓, B3 (headless disk path) ✓,
B4 (SSH keys not reaching Ignition) ✓. No open blockers for 1.0.

**VM verification (required before release tag):**
```bash
just vm      # go through the full TUI, confirm install completes and SSH works
just vm-e2e  # automated 4-pass — must exit 0 (DHCP · static · sysext · NVIDIA)
```

The most recent review record is `docs/REVIEW-2026-05-20.md`.

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
