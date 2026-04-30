# Knuckle

A modern, interactive TUI installer for [Flatcar Container Linux](https://www.flatcar.org/), designed for bare-metal deployments.

## Why

Flatcar is typically provisioned in cloud environments via Ignition configs. Bare-metal installations lack a polished setup experience. Knuckle bridges this gap with an intuitive terminal wizard that generates Ignition configurations and executes the installation.

## Features (v1 — in development)

- **Guided installation wizard** — step-by-step TUI built with [charm.sh](https://charm.sh)
- **Hardware probing** — automatic disk and network interface discovery
- **Network configuration** — DHCP or static IPv4
- **SSH key management** — manual entry or fetch from GitHub (`github.com/username.keys`)
- **System extensions** — browse and select from the official [Flatcar Bakery](https://www.flatcar.org/docs/latest/provisioning/sysext/)
- **Ignition generation** — produces valid Ignition JSON via Butane (Flatcar variant)
- **Single command install** — wraps `flatcar-install` for disk provisioning

## v1 Support Matrix

| Dimension | Supported |
|---|---|
| Architecture | x86_64 |
| Storage | Single target disk |
| Networking | DHCP, static IPv4 |
| Language | English |
| Sysexts | Official Flatcar Bakery |
| Config mode | Guided OR external Ignition URL (mutually exclusive) |

## Quick Start

```bash
# Build from source
just build

# Run the installer (on a Flatcar live environment)
./bin/knuckle

# Dry-run mode (no disk writes)
./bin/knuckle --dry-run
```

## Development

```bash
just ci          # full pipeline: tidy + lint + test + build
just test        # run tests
just lint        # golangci-lint
just fmt         # format code
just run         # go run the TUI
```

Requires: Go 1.23+, [just](https://just.systems), [golangci-lint](https://golangci-lint.run)

## Architecture

```
cmd/knuckle/         → CLI entrypoint
internal/wizard/     → step flow state machine
internal/tui/        → Bubble Tea view models
internal/probe/      → system probing (lsblk, ip, udevadm)
internal/runner/     → command execution wrapper (supports --dry-run)
internal/bakery/     → sysext catalog client
internal/ignition/   → Butane/Ignition config generation
internal/install/    → flatcar-install orchestration
internal/validate/   → input and config validation
```

## Tech Stack

- [Go](https://go.dev)
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) — TUI framework (`charm.land/bubbletea/v2`)
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) — styling (`charm.land/lipgloss/v2`)
- [Huh v2](https://github.com/charmbracelet/huh) — form inputs
- [Bubbles v2](https://github.com/charmbracelet/bubbles) — reusable components
- [Butane v0.27](https://github.com/coreos/butane) — Ignition config compilation
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/) — disk provisioning

## License

Apache 2.0
