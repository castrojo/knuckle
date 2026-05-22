# CI & Testing — knuckle

> The contract between contributors, CI, and the principal-engineer review.
> If something here disagrees with reality, **reality wins** and this doc gets
> fixed.

## Test Pyramid

```
                ┌────────────────────────────────────────┐
                │   VM e2e  (automated, 4-pass)           │   just vm-e2e
                └──────────────────┬─────────────────────┘
                 ┌─────────────────┴──────────────────────┐
                 │  Headless e2e  (config generation)     │   just headless-test
                 └────────────────┬───────────────────────┘
              ┌─────────────────── ┴───────────────────────┐
              │  Integration  (network, off-CI)             │  go test -tags=integration
              └──────────────────┬──────────────────────────┘
              ┌──────────────────┴───────────────────────────┐
              │  Golden  (internal/ignition)                  │  go test ... -update
              └──────────────────┬────────────────────────────┘
        ┌─────────────────────── ┴─────────────────────────────┐
        │  Unit  (internal/**)                                  │  go test -race ./...
        └───────────────────────────────────────────────────────┘
```

| Layer       | Runs in CI? | What it covers                                                 |
| ----------- | :---------: | -------------------------------------------------------------- |
| Unit        |     ✅      | All packages, race-clean                                       |
| Golden      |     ✅      | Ignition output stability (regenerate with `-update`)          |
| Headless e2e|     ✅      | `just headless-test` — build + canned JSON, validates config generation |
| Integration |     ❌      | Tagged `//go:build integration`. Real HTTP to GitHub + Flatcar |
| VM e2e      |     ❌      | Requires QEMU + KVM; dev-machine only; automated 4-pass recipe |

## Coverage Gate

`just cover-check` enforces per-package thresholds. Current numbers as of
2026-05-20:

| Package              | Now   | Gate | Aspiration (TEST-PLAN.md) |
| -------------------- | ----- | ---- | ------------------------- |
| `internal/model`     | 100%  | 90%  | ≥ 90%                     |
| `internal/iso`       | 100%  | 70%  | (n/a)                     |
| `internal/validate`  |  97%  | 85%  | ≥ 95%                     |
| `internal/ignition`  |  93%  | 85%  | ≥ 90%                     |
| `internal/github`    |  90%  | 85%  | (n/a)                     |
| `internal/headless`  |  87%  | 70%  | (n/a)                     |
| `internal/bakery`    |  85%  | 80%  | ≥ 85%                     |
| `internal/runner`    |  81%  | 80%  | ≥ 80%                     |
| `internal/probe`     |  81%  | 80%  | ≥ 85%                     |
| `internal/wizard`    |  81%  | 70%  | ≥ 85%                     |
| `internal/install`   |  76%  | 70%  | ≥ 80%                     |
| `internal/tui`       |  52%  | 40%  | ≥ 70%                     |

Gates are set conservatively below current numbers so CI fails on
**regression**, not on aspirational drift. When a package's actual coverage
rises and stays there, raise the gate in `Justfile :: cover-check`.

## CI Workflows

### `.github/workflows/ci.yml`

| Job            | What it does                                                            | Required to merge |
| -------------- | ----------------------------------------------------------------------- | :---------------: |
| `build-test`   | `go mod tidy` (clean), `gofmt`, `go vet`, `go build`, `go test -race`  |        ✅         |
| `lint`         | `golangci-lint run` (v2.11.4 via GHA action)                           |        ✅         |
| `vuln`         | `go tool govulncheck ./...` (version pinned in `go.mod`)               |        ✅         |
| `coverage`     | `just cover-check` + uploads `cover.out` artifact (14-day retention)   |        ✅         |
| `headless-e2e` | `just headless-test` — build + canned JSON config, validates config generation |        ✅         |

**Tool version pinning:** `govulncheck` is pinned in `go.mod` via `go tool`.
`golangci-lint` is pinned in `Justfile::GOLANGCI_LINT_VERSION` (local) and
`ci.yml::golangci-lint-action version` (CI). Bump both together.

Concurrency: per-ref, with `cancel-in-progress: true` — pushes to the same
branch cancel earlier in-flight runs.

Permissions: `contents: read` at workflow scope. Each job specifies its own
needs. `persist-credentials: false` on every `actions/checkout` — keeps the
`GITHUB_TOKEN` out of the working directory.

### `.github/workflows/security.yml`

| Job                 | When                          | What                                          |
| ------------------- | ----------------------------- | --------------------------------------------- |
| `codeql`            | push, PR, weekly cron         | CodeQL Go scan, `security-and-quality` suite  |
| `dependency-review` | PR only                       | Block PRs that introduce high-severity CVEs   |
| `scorecard`         | push to main, weekly cron     | OSSF Scorecard → SARIF upload to code scanning |

Weekly cron is Mondays at 06:37 UTC — odd minute on purpose, avoids the
00/30 GitHub Actions stampede.

### `.github/workflows/release.yml`

Builds the binary + installer ISO on `v*` tags, publishes a GitHub Release
with SHA256 sidecars. See `scripts/build-iso-ci.sh` for the ISO recipe used
in CI (`grub-mkstandalone` path). Local builds use `scripts/build-iso.sh`.

## Local Reproduction

Everything CI does is reachable from `just`:

```bash
just ci          # full pre-push gate
just fmt-check   # mirrors CI gofmt step
just vuln        # govulncheck (installs to $GOBIN)
just cover-check # per-package thresholds
```

If `just ci` passes locally but fails in CI, the gap is one of:
- Go version drift (CI pins `1.26`; bump locally with `go env GOROOT`)
- A network-dependent test running unintentionally (check for missing
  `//go:build integration` tag)
- An untracked file in your checkout that CI doesn't see

## Adding a Test

- Unit tests live next to the code (`foo.go` → `foo_test.go`).
- Fixtures go in the package's local `testdata/` (compiler ignores it).
- Golden files use the `-update` pattern: `go test ./internal/ignition -update`.
  Commit the rewritten `*.golden.json` deliberately; review the diff.
- Don't reach for the network in a unit test. Use `httptest.NewServer` or a
  `SpyRunner` stub. Integration tests that hit real APIs go behind
  `//go:build integration`.

## Adding a CI Job

- Pin the action by version, not `@main`.
- Set `permissions:` at job scope, request the minimum.
- Set `persist-credentials: false` on `actions/checkout`.
- Add it to the matrix in this doc and in `AGENTS.md`'s checklist.

## VM e2e Passes

`just vm-e2e` runs four automated passes in sequence inside QEMU (no manual
intervention required). Each pass builds a fresh overlay image, installs via
`knuckle --headless --config`, boots the installed system, and SSH-verifies the result.

| Pass          | Config                                    | Verifies                                              | Timeout |
| ------------- | ----------------------------------------- | ----------------------------------------------------- | ------- |
| DHCP          | DHCP networking, hostname, update groups  | hostname, OEM update strategy, locksmith enabled      | 15m     |
| Static        | Static IP 10.0.2.15/24, gw 10.0.2.2      | `/etc/systemd/network/10-static.network` content      | 15m     |
| Sysext        | DHCP + docker sysext                      | `/etc/extensions/docker.raw` present, `docker version` runs | 25m |
| NVIDIA        | DHCP + NVIDIA GPU config (emulated)       | `/etc/sysupdate.d/nv-*.conf` present, kernel module config | 15m |

The static pass uses QEMU's slirp NAT subnet so SSH port-forwarding still works
even with a static IP configured inside the VM. Interface name is currently
hardcoded to `ens3` — may need `eth0` on some Flatcar versions (open issue).

## Hardware-like Repro

`just hardware-repro` is the closest local reproduction path for "boot the
installer ISO, then fail inside `flatcar-install`" without needing bare metal.
It boots the built installer ISO with:

- `q35` machine type
- UEFI (`OVMF_CODE.fd` + writable `OVMF_VARS.fd`)
- SATA target disk (`ide-hd`, serial `target-disk`)
- `e1000e` NIC
- QEMU `fw_cfg` Ignition so SSH is available in the live installer

The recipe runs `knuckle --headless` inside that live ISO environment so it
exercises the same `internal/install` handoff as the TUI, while making failure
capture deterministic. On every run it writes:

- `.vm/hardware-install-output.log` — CLI output from the headless install run
- `.vm/hardware-knuckle.log` — `/tmp/knuckle.log` from the live installer
- `.vm/hardware-journal.log` — selected system journal from the live boot
- `.vm/hardware-disk-inventory.log` — `lsblk` plus `/dev/disk/by-id` inventory
- `.vm/hardware-installer-serial.log` — VM serial console output

## Roadmap

Tracked in `docs/REVIEW-2026-05-19.md` (passes 1-2) and session notes from
2026-05-20 (passes 3-4). Highlights:

- ~~Promote `just headless-test` into a CI job~~ — **done** (2026-05-19,
  `headless-e2e` job in `ci.yml`).
- ~~SSH keys silently dropped in headless path~~ — **fixed** (2026-05-20,
  `resolveSysexts()` + `StepUser` handler).
- ~~Sysexts silently dropped in headless path~~ — **fixed** (2026-05-20,
  `resolveSysexts()` added to headless `Run()`).
- ~~Goroutine leak on force-quit during install~~ — **fixed** (2026-05-20,
  `installCancel` stored in Model, called on all quit paths).
- ~~`just vm` required manual reboot step~~ — **fixed** (2026-05-20, `just vm`
  now boots installed system automatically after knuckle exits).
- ~~Blocker B1~~ GPG signature verification — **closed**. `internal/bakery/verify.go` validates Flatcar release `.DIGESTS.asc` against the embedded signing key. Separate from knuckle's own release artifact signing (cosign keyless via Sigstore in `release.yml`).
- ~~Blocker B2~~ Reboot via runner — **closed**.
- ~~Blocker B3~~ `validate.DiskPath()` in headless — **closed**.
- ~~Blocker B4~~ SSH keys reaching Ignition — **closed** (2026-05-20).
- Land `FuzzHostname`, `FuzzCIDR`, `FuzzSSHKey` and run with `-fuzztime=30s`
  in a nightly job.
- N-SEC1 MEDIUM: add max-length check on sysext download URLs in `bakery.go`.
- Verify `ens3` vs `eth0` interface name in static-network vm-e2e pass.
- Add fixture gaps: `lsblk-empty.json`, `lsblk-all-removable.json`,
  `ip_addr-ipv6-only.json`, `bakery-malformed-digests` (from QA review).
- Raise `tui` coverage (currently 52%) by extracting more pure logic into
  `wizard` / `validate`.
