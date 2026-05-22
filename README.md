# Knuckle

[![codecov](https://codecov.io/gh/projectbluefin/knuckle/graph/badge.svg)](https://codecov.io/gh/projectbluefin/knuckle)

A modern, interactive TUI "installer" for [Flatcar Container Linux](https://www.flatcar.org/), designed for bare-metal deployments. Not a real installer because making one would be dumb. There's no reason to make a "Bluefin Server" if Bluefin just gives you a nice tool to use Flatcar. 

Knuckle is a charm.sh form that makes a valid ignition file and passes it to the Flatcat installer. We're just making users not have to use ignition, with an ubuntu-server style install UX. 

This is also basically [Azure Container Linux (Home Edition)](https://opensource.microsoft.com/blog/2026/05/18/from-open-source-to-agentic-systems-microsoft-at-open-source-summit-north-america-2026/). 
- [Download an ISO](https://github.com/projectbluefin/knuckle/releases) for ARM or AMD64 and install it.
- Follow the installer you'll be left with a pristine upstream install of Flatcar linux
- This is early and prealpha, all feedback and contributions welcome!

![img](https://github.com/user-attachments/assets/802bb450-f48c-4186-a1d0-542535124bc5)

## Why

Another Linux for the home? Flatcar is in the [CNCF](https://cncf.io), which means there's a core operating system in a vendor-neutral Foundation. The kind of tech that will stick around in the long term. And designed to run anything you want. Plop anything from [linuxserver.io](https://www.linuxserver.io/) right on it for an awesome setup. 

Perfect for home servers, NAS builds, k8s cluster setups, you name it. The [Flatcar Bakery](https://flatcar.github.io/sysext-bakery/) is included, allowing for endless extensibility:

![sysexts](https://github.com/user-attachments/assets/86f0c439-4c27-4c04-a2fc-880d499f7c28)
![progress](https://github.com/user-attachments/assets/0d4160da-8731-4a92-b103-98b901dd9c3d)


## Features

- **9-step guided wizard** — Welcome → Network → Storage → User → Sysext → Update Strategy → Review → Install → Done
- **Channel selector with version details** — shows kernel, systemd, docker, containerd, ignition, and etcd versions per channel (sourced from SBOM JSON, `version.txt`, package lists, and `rootfs-included-sysexts`)
- **Hardware probing** — automatic disk and network interface discovery with `/dev/disk/by-id` path resolution
- **Network configuration** — DHCP or static IPv4
- **User setup** — hostname, timezone, password (bcrypt hashed), GitHub SSH key fetching with multi-key support, local `~/.ssh/*.pub` auto-detection
- **System extensions** — architecture-aware sysext catalog fetched via TLS from [flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery) GitHub Releases API (not yet cryptographically verified — see [docs/SECURITY.md](docs/SECURITY.md))
- **Update strategy** — reboot, off, or etcd-lock options
- **Review screen** — full Butane YAML preview before install
- **Install step** — progress bar, wipes stale disk signatures, runs `flatcar-install`, then relocates the backup GPT header for larger target disks
- **Ignition generation** — produces valid Ignition JSON via in-process Butane compilation (Flatcar variant, no CLI dependency)
- **Headless mode** — `--headless --config <file.json>` for automated installs (CI/CD friendly)
- **Installer ISO** — UEFI-bootable ISO with systemd-boot (amd64 + arm64), with `systemd.gpt_auto=0` set in both boot entries to avoid GPT auto-generator hangs when the ISO is written to larger USB media
- **Config validation** — consistency checks before install
- **Dry-run mode** — `--dry-run` flag skips all disk writes
- **Ctrl+C double-press** — confirmation before quitting

## Support Matrix

| Dimension | Supported |
|---|---|
| Architecture | x86_64, ARM64 |
| Storage | Single target disk |
| Networking | DHCP, static IPv4 |
| Language | English |
| Sysexts | Official Flatcar Bakery (via GitHub Releases API, arch-aware) |
| Config mode | Guided OR external Ignition URL (`Ctrl+A`) — mutually exclusive |

## Quick Start

```bash
# Build from source (amd64)
just build

# Cross-compile for arm64
just build-arm64

# Run the installer (on a Flatcar live environment)
./bin/knuckle

# Dry-run mode (no disk writes)
./bin/knuckle --dry-run

# Headless install from JSON config
./bin/knuckle --headless --config install.json
```

## Development

```bash
just              # list all recipes
just ci           # full CI: tidy + fmt + vet + lint + vuln + test-race + cover + headless-e2e + build
just build        # GOOS=linux GOARCH=amd64 CGO_ENABLED=0 → bin/knuckle
just build-arm64  # cross-compile arm64 → bin/knuckle-arm64
just test         # go test ./...
just vuln         # govulncheck ./...
just cover        # coverage profile + summary
just cover-check  # per-package coverage threshold gate
just headless-test  # build + canned JSON config (CI gate, runs on host)
```

### VM Testing

```bash
just vm           # real install in QEMU → auto-boots installed system after
just vm-e2e       # automated: headless install → boot → verify SSH + hostname (4 passes)
just hardware-repro # installer ISO in a hardware-like VM, captures install failure artifacts
just boot-iso     # build ISO → boot in QEMU GTK window
just e2e          # build ISO → boot → interactive install
just ssh          # SSH into running VM
```

`just vm` downloads a Flatcar stable QEMU image, boots a VM with two disks (boot + target), SCPs the binary in, and launches knuckle over SSH. After install completes, it kills the installer VM and boots from the installed target disk to verify SSH works.

`just vm-e2e` is fully automated — runs 4 passes (DHCP, static network, docker sysext, NVIDIA), verifying hostname, OS version, update strategy, and sysext activation on each.

`just hardware-repro` boots the installer ISO with `q35`, UEFI, a SATA target disk, and an `e1000e` NIC so the VM looks more like a generic physical machine than a paravirtualized guest. It then runs the same `internal/install` path via `--headless` and saves the useful artifacts under `.vm/` (`hardware-install-output.log`, `hardware-knuckle.log`, `hardware-journal.log`, `hardware-disk-inventory.log`, `hardware-installer-serial.log`).

ARM64 VM testing: `KNUCKLE_ARCH=arm64 just vm` (requires native arm64 hardware or QEMU TCG).

Requires: Go 1.26+, [just](https://just.systems), QEMU with KVM

## Architecture

```
cmd/knuckle/         → CLI entrypoint, flag parsing, runner wiring
internal/bakery/     → sysext catalog + Flatcar release/SBOM fetchers, SHA512+GPG verification
internal/github/     → SSH key fetch + GitHub Releases API client
internal/headless/   → --headless --config JSON-driven install path
internal/ignition/   → Butane assembly + in-process Butane→Ignition compilation
internal/install/    → flatcar-install orchestration via runner
internal/iso/        → installer ISO builder helpers
internal/model/      → shared data types (InstallConfig, DiskInfo, NetworkInterface)
internal/probe/      → lsblk + ip addr JSON parsing, /dev/disk/by-id resolution
internal/runner/     → Runner interface: RealRunner, DryRunner, SpyRunner
internal/tui/        → Bubble Tea view models (one sub-model per step)
internal/validate/   → hostname, CIDR, gateway, SSH key, timezone, disk path validators
internal/wizard/     → step state machine, navigation, validation gates
```

### Key Design Decisions

- **Runner abstraction** — every external command goes through `internal/runner.Runner`. Three implementations: `RealRunner` (prod), `DryRunner` (no-op + logging), `SpyRunner` (test recorder). Reboot is injected via `rebootFn`.
- **Flatcar Butane variant** — `variant: flatcar`, compiled in-process via `github.com/coreos/butane` v0.27+. No `butane` CLI needed on the target system.
- **Architecture-aware** — `InstallConfig.Arch` is set from `runtime.GOARCH` (compile-time constant). Sysext catalog, channel fetches, and ISO builds all parameterize on arch. LTS channel is guarded for arm64 (not published by Flatcar).
- **Sysext catalog** — from [flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery) GitHub Releases API; selects `x86-64.raw` or `arm64.raw` assets based on target arch.
- **Channel versions** — assembled from SBOM JSON (preferred), `version.txt`, package lists, and `rootfs-included-sysexts`.
- **Supply-chain verification** — SHA512 digest check + GPG signature verification against embedded Flatcar signing key.
- **Disk identity** — `/dev/disk/by-id` preferred; never trusts `/dev/sdX` enumeration order.
- **Headless mode** — mirrors TUI path through the same `internal/install` package. JSON config schema with `arch` field.
- **ISO injection** — modifies Flatcar's `usr.squashfs` directly (the only reliable method for Flatcar PXE live boot). Uses systemd-boot (UEFI-only, BLS entries).

## Tech Stack

- [Go](https://go.dev) 1.26+ (CGO_ENABLED=0, static binary)
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) — styling
- [Huh v2](https://github.com/charmbracelet/huh) — form inputs (Dracula theme)
- [Bubbles v2](https://github.com/charmbracelet/bubbles) — reusable components
- [Butane v0.27](https://github.com/coreos/butane) — Ignition config compilation (in-process)
- [ProtonMail/go-crypto](https://github.com/ProtonMail/go-crypto) — GPG signature verification
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/) — disk provisioning

## CI/CD

- **CI** (`ci.yml`) — go mod tidy, gofmt, vet, golangci-lint, govulncheck, test -race, per-package coverage gate, headless e2e, arm64 cross-compile
- **Security** (`security.yml`) — CodeQL (Go), OSSF Scorecard, dependency-review
- **Release** (`release.yml`) — triggered on `v*` tags; builds amd64 on `ubuntu-latest`, arm64 on `ubuntu-24.04-arm`; produces binaries + installer ISOs + cosign bundles

## Community

- [Flatcar on Discord](https://flatcar.org/discord) — chat with the Flatcar community
- [Flatcar docs](https://www.flatcar.org/docs/) — official documentation
- [knuckle issues](https://github.com/projectbluefin/knuckle/issues) — file bugs and ideas

## License

Apache 2.0
