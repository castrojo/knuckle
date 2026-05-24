# Contributing to Knuckle

Thanks for contributing! Knuckle is early and pre-alpha — all feedback and PRs are welcome.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| [Go](https://go.dev) | 1.26+ | `go version` to check |
| [just](https://just.systems) | any | `cargo install just` or `brew install just` |
| QEMU + KVM | any | Optional — required for VM/ISO testing only |

Install QEMU on Ubuntu:
```bash
sudo apt install qemu-kvm qemu-system-x86 qemu-system-arm ovmf
```

## Getting Started

```bash
git clone https://github.com/projectbluefin/knuckle
cd knuckle

# Run the full CI gate (no QEMU needed)
just ci
```

`just ci` runs: `go mod tidy` check → `gofmt` → `go vet` → `golangci-lint` → `govulncheck` → `go test -race` → coverage gate → headless e2e.

If `just ci` passes locally, your change is ready for a PR.

## Optional: VM Testing

```bash
just vm          # real install in QEMU, boots installed system after
just vm-e2e      # 4-pass automated: DHCP, static, docker sysext, NVIDIA
just boot-iso    # boot installer ISO in QEMU GTK window
```

Requires QEMU/KVM. See [docs/CI-AND-TESTING.md](docs/CI-AND-TESTING.md) for the full test pyramid.

## Submitting a PR

1. **Fork** and create a branch from `main`
2. **Make your change** — keep scope tight; see `## Scope` in the PR template
3. **Run** `just ci` — it must pass before you open a PR
4. **Sign commits** — DCO required: `git commit -s`
5. **Fill out the PR template** — link the issue, check the boxes

```bash
# Sign a commit
git commit -s -m "fix(tui): correct tab focus order in StepNetwork"

# Sign an existing commit
git commit --amend -s
```

## Code Style

- `gofmt` enforced by CI — run `just fmt` before committing
- `golangci-lint` enforced — run `just lint` to check locally
- No CGO — keep `CGO_ENABLED=0`
- New packages go under `internal/` — nothing exported from `internal/` is public API

## Tests

- Unit tests live next to the code: `foo.go` → `foo_test.go`
- Golden files use `-update`: `go test ./internal/ignition -update` — commit the result deliberately
- Network-dependent tests go behind `//go:build integration`
- No `os.Exec` or network calls in unit tests — use `SpyRunner` or `httptest.NewServer`

See [docs/CI-AND-TESTING.md](docs/CI-AND-TESTING.md) for coverage gates and the full test pyramid.

## Architecture Overview

```
cmd/knuckle/     → entrypoint, flag parsing
internal/tui/    → Bubble Tea step models (one per wizard step)
internal/wizard/ → step state machine, navigation, validation gates
internal/install/→ flatcar-install orchestration
internal/headless/ → --headless --config path (mirrors TUI)
internal/ignition/ → Butane assembly + in-process compilation
```

Full architecture in [README.md#architecture](README.md#architecture).

## Security Issues

Do **not** open a public issue for vulnerabilities. Use [GitHub Security Advisories](https://github.com/projectbluefin/knuckle/security/advisories) instead. See [docs/SECURITY.md](docs/SECURITY.md).

## Community

- [Flatcar on Discord](https://flatcar.org/discord)
- [Issues](https://github.com/projectbluefin/knuckle/issues) for bugs and feature requests
