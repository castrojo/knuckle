# Knuckle

[![codecov](https://codecov.io/gh/projectbluefin/knuckle/graph/badge.svg)](https://codecov.io/gh/projectbluefin/knuckle) [![Go Report Card](https://goreportcard.com/badge/github.com/projectbluefin/knuckle)](https://goreportcard.com/report/github.com/projectbluefin/knuckle)

A modern, interactive TUI "installer" for [Flatcar Container Linux](https://www.flatcar.org/), designed for bare-metal deployments. Not a real installer because making one would be dumb. There's no reason to make a "Bluefin Server" if Bluefin just gives you a nice tool to use Flatcar. But also it's just an ignition thing, so let's **UNIFY COREOS AND FLATCAR INSTALLATION** and share one installer and tests.

Knuckle is a charm.sh form that makes a valid ignition file and passes it to the Flatcar installer. We're just making users not have to use ignition, with an ubuntu-server style install UX. 

This is also basically [Azure Container Linux (Home Edition)](https://opensource.microsoft.com/blog/2026/05/18/from-open-source-to-agentic-systems-microsoft-at-open-source-summit-north-america-2026/). 
- [Download an ISO](https://github.com/projectbluefin/knuckle/releases) for ARM or AMD64 and install it.
- Follow the installer you'll be left with a pristine upstream install of Flatcar linux
- This is early and prealpha, all feedback and contributions welcome!

![img](https://github.com/user-attachments/assets/802bb450-f48c-4186-a1d0-542535124bc5)

> [!WARNING]
> **Pre-alpha software — use with caution**
> - **Wipes the target disk** — the installer is destructive with no undo after confirmation
> - **UEFI-only** — BIOS/legacy boot is not supported
> - **Single disk only** — multi-disk setups are not supported
> - **English only** — no i18n support yet

## Installation

> Knuckle is pre-alpha. Test on hardware you can reinstall.

### 1. Download the installer ISO

From [Releases](https://github.com/projectbluefin/knuckle/releases), download:

| Architecture | Release asset |
|---|---|
| AMD64 / x86_64 | `knuckle-installer-stable-amd64.iso` |
| ARM64 | `knuckle-installer-stable-arm64.iso` |

Verify the checksum (recommended):

```bash
# use the matching .sha256 file for your architecture
sha256sum -c knuckle-installer-stable-amd64.iso.sha256
```

### 2. Write the ISO to USB

Linux:

```bash
# Replace /dev/sdX with your USB device (check with lsblk first)
sudo dd if=knuckle-installer-stable-amd64.iso of=/dev/sdX bs=4M status=progress conv=fsync
```

macOS: use `diskutil list` to find your USB device and write with `dd` using the matching `/dev/diskN` path.

Windows: use [Rufus](https://rufus.ie), select the ISO, choose **DD Image** mode, then start.

### 3. Boot from USB

1. Insert the USB drive into the target machine.
2. Enter BIOS/UEFI setup (commonly `F2`, `F12`, or `Del` at power-on).
3. Set USB as first boot device, or pick it from the one-time boot menu.
4. Save and reboot; Knuckle starts automatically.

### 4. Follow the wizard

The base flow is 9 steps:

1. **Welcome** (`Ctrl+A` toggles external Ignition URL mode)
2. **Network** (DHCP or static IPv4)
3. **Storage** (select target disk; all data is erased)
4. **User** (hostname, timezone, password, SSH keys)
5. **Sysext** (optional Flatcar Bakery extensions)
6. **Update strategy**
7. **Review** (`Ctrl+B` toggles full Butane preview)
8. **Install** (`flatcar-install` runs with live progress)
9. **Done** (remove USB and reboot)

If you enable Tailscale or NVIDIA options, additional wizard steps appear.

### 5. First boot

SSH to the installed system with the credentials you configured:

```bash
ssh core@<your-server-ip>
```

For unattended installs, see [docs/HEADLESS-CONFIG.md](docs/HEADLESS-CONFIG.md).

## Why

Another Linux for the home? Flatcar is in the [CNCF](https://cncf.io), which means there's a core operating system in a vendor-neutral Foundation. The kind of tech that will stick around in the long term. And designed to run anything you want. Plop anything from [linuxserver.io](https://www.linuxserver.io/) right on it for an awesome setup. 

Perfect for home servers, NAS builds, k8s cluster setups, you name it. The [Flatcar Bakery](https://flatcar.github.io/sysext-bakery/) is included, allowing for endless extensibility:

![sysexts](https://github.com/user-attachments/assets/86f0c439-4c27-4c04-a2fc-880d499f7c28)
![progress](https://github.com/user-attachments/assets/0d4160da-8731-4a92-b103-98b901dd9c3d)


## Features

- **Up to 11-step guided wizard** — Welcome → Network → Storage → User → Sysext → [Nvidia] → [Tailscale] → Update Strategy → Review → Install → Done (Nvidia and Tailscale steps are conditional on sysext selection)
- **Channel selector with version details** — shows kernel, systemd, docker, containerd, ignition, and etcd versions per channel (sourced from SBOM JSON, `version.txt`, package lists, and `rootfs-included-sysexts`)
- **Hardware probing** — automatic disk and network interface discovery with `/dev/disk/by-id` path resolution
- **Network configuration** — DHCP or static IPv4
- **User setup** — hostname, timezone, password (bcrypt hashed), GitHub SSH key fetching with multi-key support, local `~/.ssh/*.pub` auto-detection
- **System extensions** — architecture-aware sysext catalog fetched via TLS from [flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery) GitHub Releases API (not yet cryptographically verified — see [docs/SECURITY.md](docs/SECURITY.md))
- **NVIDIA driver series** — conditional step shown when `nvidia-runtime` sysext is selected; selects the kernel driver series (e.g. `550`, `560`) matching the chosen NVIDIA runtime sysext; supported in both TUI and headless (`nvidiaDriverSeries`) modes
- **Tailscale provisioning** — conditional step shown when `tailscale` sysext is selected; sets a pre-auth key (`tskey-auth-…`) written to `/etc/tailscale/tailscale.env`; supported in both TUI and headless (`tailscaleAuthKey`) modes
- **Update strategy** — reboot, off, or etcd-lock options
- **Review screen** — full Butane YAML preview before install
- **Install step** — progress bar, wipes stale disk signatures, runs `flatcar-install`, then relocates the backup GPT header for larger target disks
- **Ignition generation** — produces valid Ignition JSON via in-process Butane compilation (Flatcar variant, no CLI dependency)
- **Headless mode** — `--headless --config <file.json>` for automated installs (CI/CD friendly)
- **Installer ISO** — UEFI-bootable ISO with systemd-boot (amd64 + arm64), with `systemd.gpt_auto=0` set in both boot entries to avoid GPT auto-generator hangs when the ISO is written to larger USB media
- **Config validation** — consistency checks before install
- **Swap file** — 4 GiB `/var/swapfile` provisioned by default; configurable size (up to 32 GiB) or disabled via headless config
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

## Keyboard Shortcuts

- `Ctrl+A` on the Welcome step — toggle external Ignition URL mode
- `Ctrl+B` on the Review step — toggle the Butane YAML preview
- `Ctrl+C` × 2 — quit with confirmation

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

# Pin to a specific Flatcar release
./bin/knuckle --channel beta --flatcar-version 3975.2.1
```

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--dry-run` | false | Simulate install — no disk writes |
| `--headless` | false | Non-interactive mode; requires `--config` |
| `--config <file.json>` | — | Path to JSON config for headless installs |
| `--channel <name>` | stable | Flatcar release channel: `stable`, `beta`, `alpha`, `lts`, `edge` |
| `--flatcar-version <ver>` | — | Pin to a specific Flatcar version (e.g. `3510.2.8`); omit to use latest |
| `--log-file <path>` | `/tmp/knuckle.log` | Path to structured log file (`slog`); stdout is reserved for the TUI |
| `--demo` | false | Mock hardware and catalog data — no network, no real disks; for UI demos and screen recording |
| `--version` | — | Print version and exit |

## Environment Variables

| Variable | Example | Effect |
|---|---|---|
| `KNUCKLE_ARCH` | `KNUCKLE_ARCH=arm64 just vm` | Overrides the default `amd64` target used by `just` recipes so builds, ISO tasks, and VM workflows target arm64 instead. Useful for ARM64 cross-builds and VM testing. |
| `KNUCKLE_TEST_MAIN` | `KNUCKLE_TEST_MAIN=1 go test ./cmd/knuckle` | Internal test helper used by `cmd/knuckle/main_test.go` to re-enter `main()` inside the compiled test binary, so flag-validation and early-exit paths can be exercised without launching the TUI. |

## Headless Mode

`--headless --config <file.json>` drives the same install path as the TUI without any user interaction. Useful for CI/CD, automated bare-metal provisioning, and scripted lab setups.

Minimal example (`install.json`):

```json
{
  "hostname": "flatcar-01",
  "channel": "stable",
  "disk": "/dev/disk/by-id/ata-SomeSeagate_1TB",
  "network": {"mode": "dhcp"},
  "users": [
    {"username": "core", "ssh_keys": ["ssh-ed25519 AAAA..."]}
  ],
  "update_strategy": "reboot"
}
```

Swap is **enabled by default** (4 GiB `/var/swapfile`). To customise:

```json
{
  "hostname": "flatcar-01",
  "channel": "stable",
  "disk": "/dev/disk/by-id/ata-SomeSeagate_1TB",
  "network": {"mode": "dhcp"},
  "users": [
    {"username": "core", "ssh_keys": ["ssh-ed25519 AAAA..."]}
  ],
  "update_strategy": "reboot",
  "swap": {"enabled": true, "size_mb": 2048}
}
```

To disable swap entirely:

```json
{
  "hostname": "flatcar-01",
  "channel": "stable",
  "disk": "/dev/disk/by-id/ata-SomeSeagate_1TB",
  "network": {"mode": "dhcp"},
  "users": [
    {"username": "core", "ssh_keys": ["ssh-ed25519 AAAA..."]}
  ],
  "update_strategy": "reboot",
  "swap": {"enabled": false}
}
```

For the full field reference see [docs/HEADLESS-CONFIG.md](docs/HEADLESS-CONFIG.md).

Valid `swap.size_mb` range: 1–32768 (MiB). Omitting `swap` or setting `size_mb: 0` uses the 4 GiB default.

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

ARM64 VM testing: `KNUCKLE_ARCH=arm64 just vm` (requires native arm64 hardware or QEMU TCG). Set the same env var for other arm64-targeted `just` recipes such as `build`, `vm-e2e`, `iso`, and `boot-iso`.

**Prerequisites:**
- **Go 1.26+** and **[just](https://just.systems)**
- **QEMU with KVM:**
  - Ubuntu/Debian: `apt install qemu-system-x86-64 qemu-efi-aarch64 ovmf`
  - Fedora/RHEL: `dnf install qemu-system-x86 qemu-system-aarch64 edk2-ovmf`
  - Arch: `pacman -S qemu qemu-efi-aarch64 edk2-ovmf`
- **OVMF firmware** — required for `just hardware-repro` (auto-installed above) and UEFI VM testing
- **Flatcar QEMU base image** — download via [docs/GHOST-LAB.md](docs/GHOST-LAB.md) before first run. `just vm` will fail if the image is missing

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
- **Flatcar Butane variant** — `variant: flatcar`, compiled in-process via `github.com/coreos/butane` v0.28+. No `butane` CLI needed on the target system.
- **Architecture-aware** — `InstallConfig.Arch` is set from `runtime.GOARCH` (compile-time constant). Sysext catalog, channel fetches, and ISO builds all parameterize on arch. LTS channel is guarded for arm64 (not published by Flatcar).
- **Sysext catalog** — from [flatcar/sysext-bakery](https://github.com/flatcar/sysext-bakery) GitHub Releases API; selects `x86-64.raw` or `arm64.raw` assets based on target arch.
- **Channel versions** — assembled from SBOM JSON (preferred), `version.txt`, package lists, and `rootfs-included-sysexts`.
- **Supply-chain verification** — SHA512 digest check + GPG signature verification against embedded Flatcar signing key.
- **Disk identity** — `/dev/disk/by-id` preferred; never trusts `/dev/sdX` enumeration order.
- **Headless mode** — mirrors TUI path through the same `internal/install` package. JSON config schema with `arch` field.
- **ISO injection** — modifies Flatcar's `usr.squashfs` directly (the only reliable method for Flatcar PXE live boot). Uses systemd-boot (UEFI-only, BLS entries).

## Tech Stack

- [Go](https://go.dev) 1.26+ (CGO_ENABLED=0, static binary)
- [Bubble Tea v2.0.6](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss v2.0.3](https://github.com/charmbracelet/lipgloss) — styling
- [Huh v2.0.3](https://github.com/charmbracelet/huh) — form inputs (Dracula theme)
- [Bubbles v2.1.0](https://github.com/charmbracelet/bubbles) — reusable components
- [Butane v0.28](https://github.com/coreos/butane) — Ignition config compilation (in-process)
- [ProtonMail/go-crypto](https://github.com/ProtonMail/go-crypto) — GPG signature verification
- [flatcar-install](https://www.flatcar.org/docs/latest/installing/bare-metal/installing-to-disk/) — disk provisioning

## CI/CD

- **CI** (`ci.yml`) — go mod tidy, gofmt, vet, golangci-lint, govulncheck, test -race, per-package coverage gate, headless e2e, arm64 cross-compile
- **Security** (`security.yml`) — CodeQL (Go), OSSF Scorecard, dependency-review
- **Release** (`release.yml`) — triggered on `v*` tags; builds amd64 on `ubuntu-latest`, arm64 on `ubuntu-24.04-arm`; produces binaries + installer ISOs + cosign bundles

## After Install

Once knuckle finishes, the machine reboots into a running Flatcar system. Here's what to do next.

**Log in** — SSH as the `core` user with the key(s) you provided during setup:

```bash
ssh core@<ip-or-hostname>
```

No password login by default; the key you entered (or fetched from GitHub) is the only way in.

**Check the system** — Flatcar is an immutable OS. The root filesystem is read-only; user state lives under `/var`, `/etc`, and `/home`. To see what's running:

```bash
systemctl list-units --type=service
journalctl -b          # boot logs
cat /etc/os-release    # OS version
```

**System extensions (sysexts)** — if you installed Docker, Kubernetes, or other sysexts, they land in `/etc/extensions/` and activate on the next boot (or immediately via `systemd-sysext refresh`). Check status:

```bash
systemd-sysext status
docker version          # if docker sysext was installed
```

**Updates** — Flatcar auto-updates by default. The update strategy you chose controls how reboots happen:

| Strategy | Behaviour |
|---|---|
| `reboot` | Reboots automatically when an update lands |
| `etcd-lock` | Co-ordinates reboots across a cluster via locksmith |
| `off` | Downloads updates but never reboots — you reboot manually |

Check update status: `update_engine_client -status`

**Install more software** — the read-only root means traditional package managers don't apply. Options:
- **Sysexts** — system-level tools (Docker, Kubernetes, Tailscale, NVIDIA) via the [Flatcar Bakery](https://flatcar.github.io/sysext-bakery/)
- **Containers** — run workloads with Docker or another container runtime
- **Toolbox / distrobox** — `toolbox enter` drops you into a mutable Fedora container for ad-hoc CLI work

**Reprovisioning** — to change config (add users, switch update strategy, add sysexts), edit the Ignition/Butane source and re-run knuckle. Flatcar is designed for reprovisioning rather than in-place mutation.

## Community

- [Flatcar on Discord](https://flatcar.org/discord) — chat with the Flatcar community
- [Flatcar docs](https://www.flatcar.org/docs/) — official documentation
- [Troubleshooting guide](docs/TROUBLESHOOTING.md) — common install/first-boot fixes and diagnostic commands
- [knuckle issues](https://github.com/projectbluefin/knuckle/issues) — file bugs and ideas

## License

Apache 2.0
