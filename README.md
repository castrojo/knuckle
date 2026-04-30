# Knuckle

A modern, interactive TUI installer for [Flatcar Container Linux](https://www.flatcar.org/), designed for bare-metal deployments.

## Why

Flatcar is typically provisioned in cloud environments via Ignition configs. Bare-metal installations lack a polished setup experience. Knuckle bridges this gap with an intuitive terminal wizard that generates Ignition configurations and executes the installation.

## Status

**Feature-complete.** All 9 TUI steps are implemented, with 214 tests passing (race-clean).

## Features

- **9-step guided wizard** — Welcome → Network → Storage → User → Sysext → Update Strategy → Review → Install → Done
- **Channel selector with version details** — shows kernel, systemd, docker, containerd, ignition, and etcd versions per channel (sourced from `version.txt`, package lists, and `rootfs-included-sysexts`)
- **Hardware probing** — automatic disk and network interface discovery with `/dev/disk/by-id` path resolution
- **Network configuration** — DHCP or static IPv4
- **User setup** — hostname, timezone, password (bcrypt hashed), GitHub SSH key fetching with multi-key support
- **System extensions** — version-pinned sysext catalog fetched from GitHub Releases API ([flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery))
- **Update strategy** — reboot, off, or etcd-lock options
- **Review screen** — full Butane YAML preview before install
- **Install step** — progress bar, wraps `flatcar-install` for disk provisioning
- **Ignition generation** — produces valid Ignition JSON via Butane (Flatcar variant)
- **ISO generation** — `internal/iso` package for creating installer ISOs
- **Config validation** — consistency checks before install
- **Dry-run mode** — `--dry-run` flag skips all disk writes
- **Ctrl+C double-press** — confirmation before quitting

## Support Matrix

| Dimension | Supported |
|---|---|
| Architecture | x86_64 |
| Storage | Single target disk |
| Networking | DHCP, static IPv4 |
| Language | English |
| Sysexts | Official Flatcar Bakery (via GitHub Releases API) |
| Config mode | Guided OR external Ignition URL (mutually exclusive) |

## Quick Start

```bash
# Build from source
go build ./cmd/knuckle

# Run the installer (on a Flatcar live environment)
./knuckle

# Dry-run mode (no disk writes)
./knuckle --dry-run
```

## Development

```bash
# Build and test
go build ./cmd/knuckle
go test -race ./...

# Full CI pipeline (tidy + lint + test + build)
just ci

# VM testing — boots QEMU with Flatcar, deploys binary, SSH on port 2222
just vm

# SSH into the running VM
just ssh

# Build a self-contained installer disk image
just installer-disk
```

Requires: Go 1.26+, [just](https://just.systems), [golangci-lint](https://golangci-lint.run)

### VM Testing

`just vm` downloads a Flatcar stable QEMU image, cross-compiles knuckle for linux/amd64, boots a VM with two disks (boot + target), and launches knuckle over SSH with `--dry-run`. SSH is forwarded on `127.0.0.1:2222`. The TUI requires a PTY (the SSH `-t` flag handles this).

## Architecture

```
cmd/knuckle/         → CLI entrypoint
internal/bakery/     → sysext catalog client (GitHub Releases API)
internal/github/     → GitHub API client (SSH keys, releases)
internal/ignition/   → Butane/Ignition config generation
internal/install/    → flatcar-install orchestration
internal/iso/        → ISO image generation
internal/model/      → shared data model
internal/probe/      → system probing (lsblk, ip, udevadm)
internal/runner/     → command execution wrapper (supports --dry-run)
internal/tui/        → Bubble Tea view models (9 steps)
internal/validate/   → input and config validation
internal/wizard/     → step flow state machine
```

### Key Design Decisions

- **Sysext catalog** comes from the [flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery) GitHub Releases API, not from flatcar.org directly
- **Channel versions** are assembled from three sources: `version.txt` (kernel, systemd), package lists, and `rootfs-included-sysexts` (docker, containerd — these are sysexts since Flatcar 4081+, not base image packages)
- **Password hashing** uses bcrypt, computed client-side before Ignition generation

## Tech Stack

- [Go](https://go.dev) 1.26+
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) — styling
- [Huh v2](https://github.com/charmbracelet/huh) — form inputs
- [Bubbles v2](https://github.com/charmbracelet/bubbles) — reusable components
- [Butane v0.27](https://github.com/coreos/butane) — Ignition config compilation
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/) — disk provisioning

## License

Apache 2.0
