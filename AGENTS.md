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
just lint      # golangci-lint run
just fmt       # gofumpt
just vuln      # govulncheck
```

## Safety Rules

- **Never run real `flatcar-install` on host.** Use `--dry-run` or QEMU/loopback for testing.
- All system commands (lsblk, ip, flatcar-install) go through `internal/runner` — never `exec.Command` directly from TUI code.
- Disk selection must use `/dev/disk/by-id` where possible; display model, serial, size, transport, removable flag.

## Package Boundaries

| Package | Responsibility |
|---|---|
| `cmd/knuckle` | CLI entrypoint, flag parsing |
| `internal/wizard` | Step flow state machine, navigation logic |
| `internal/tui` | Bubble Tea view models, rendering |
| `internal/probe` | System probing (disks, network interfaces, hardware) |
| `internal/runner` | exec.Command wrapper, `--dry-run` support, output capture |
| `internal/bakery` | HTTP client for Flatcar Bakery sysext catalog |
| `internal/ignition` | Butane config assembly (Flatcar variant), Ignition compilation |
| `internal/install` | flatcar-install orchestration via runner |
| `internal/validate` | Input validation, config consistency checks |

## Architecture Decisions

1. **Runner abstraction** — All external commands go through `internal/runner`. This enables dry-run mode, test fixtures, and safe CI.
2. **Flatcar Butane variant** — Use `variant: flatcar` (not generic CoreOS) when generating Butane configs. Import via `github.com/coreos/butane/config`.
3. **Mutually exclusive config modes** — v1 supports either guided local generation OR external Ignition URL passthrough. No merge logic.
4. **Disk identity** — Use `/dev/disk/by-id` paths. Never rely on `/dev/sda` ordering.
5. **TUI ↔ logic separation** — `internal/tui` renders views; `internal/wizard` manages state transitions. No business logic in view models.

## Testing Strategy

- Unit tests with fixture data in `testdata/`
- Runner abstraction allows testing install/probe logic without real hardware
- Integration tests via QEMU/loopback (future)
- No real disk writes in CI — `--dry-run` is default in test mode
