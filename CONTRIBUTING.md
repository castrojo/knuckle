# Contributing to Knuckle

Thanks for contributing! Knuckle is early and pre-alpha — all feedback and PRs are welcome.

## Where to Start

New to knuckle? Good places to start:

- Issues labeled [`good first issue`](https://github.com/projectbluefin/knuckle/issues?q=is%3Aopen+label%3A%22good+first+issue%22) — curated for newcomers
- **`internal/validate/`** — hostname, CIDR, SSH key, timezone validators; well-isolated, no QEMU needed
- **Test coverage** — `just cover` shows packages under threshold; adding tests is always welcome
- **Doc improvements** — typos, clarifications, example fixes (no build tools needed)

For larger changes, open an issue first to align on scope before writing code.

### ARM64 development

Cross-compile for arm64 with `just build-arm64` — no arm64 hardware needed for compilation or unit tests.

`KNUCKLE_ARCH` defaults to `amd64` in the `Justfile`. Override it when you want `just` recipes to target arm64 instead:
- **Native arm64 hardware** with KVM: `KNUCKLE_ARCH=arm64 just vm`
- **QEMU TCG** (slow, x86_64 host): `sudo apt install qemu-system-arm` then `KNUCKLE_ARCH=arm64 just vm`
- **Other arm64 recipes**: `KNUCKLE_ARCH=arm64 just vm-e2e`, `KNUCKLE_ARCH=arm64 just iso`, `KNUCKLE_ARCH=arm64 just boot-iso`

CI uses native `ubuntu-24.04-arm` runners. TCG emulation is functional but significantly slower.

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

### Test-only environment variables

- `KNUCKLE_TEST_MAIN=1` — internal test helper used by `cmd/knuckle/main_test.go` to make the compiled test binary delegate into `main()`. This is only needed when working on CLI bootstrap and flag-handling tests.

- Unit tests live next to the code: `foo.go` → `foo_test.go`
- Golden files use `-update`: `go test ./internal/ignition -update` — commit the result deliberately
- Network-dependent tests go behind `//go:build integration`
- No `os.Exec` or network calls in unit tests — use `SpyRunner` or `httptest.NewServer`

### Testing a TUI step

Most TUI tests stay fast by testing the model directly instead of spinning up a full Bubble Tea program.

- Use `newTestWizard()` + `New(w)` to build a model with predictable state for the step you are working on.
- For command execution, prefer the `SpyRunner` pattern used across `internal/install` and related packages: stub command results with `runner.NewSpyRunner()`, call the code under test, then assert against `spy.Calls` instead of shelling out.
- For Ignition golden files, regenerate snapshots intentionally with `go test ./internal/ignition -update`, review the `*.golden.json` diff, and commit the updated files only when the new output is expected.
- Real network or process-behavior tests belong behind the integration build tag:
  ```go
  //go:build integration
  // +build integration
  ```
  Run them locally with `go test -tags=integration ./...` when you need the real path; they do not run in normal unit-test passes.
- `internal/tui/forms_builder_test.go` is the lightweight pattern for form construction tests: seed wizard state, call `build*Form()`, and assert the form is created and preserves important input values. These tests do not need a TTY.

Minimal step test structure:

```go
func TestKeyboard_CtrlB_ReviewTogglesPreview(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview

	m := New(w)
	m.activeForm = nil // bypass huh form when you want handleKey()/Update() directly

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	got := newModel.(*Model)

	if !got.showButane {
		t.Fatal("expected Ctrl+B to enable Butane preview")
	}
}
```

Use this split when adding coverage:
- state transitions and keyboard handling → step-focused tests such as `keyboard_test.go` or `*_step_test.go`
- form construction and seeded defaults → `forms_builder_test.go`
- real external behavior → integration tests with the `integration` build tag

See [docs/CI-AND-TESTING.md](docs/CI-AND-TESTING.md) for coverage gates and the full test pyramid.

## Demo Recording

The `demo/` directory holds the source for the animated demo GIF shown in the README:

- `demo/knuckle-demo.tape` — [VHS](https://github.com/charmbracelet/vhs) script that drives the terminal recording
- `demo/knuckle-install.cast` — [asciinema](https://asciinema.org/) cast produced by VHS
- `demo/knuckle-install.gif` — rendered GIF embedded in the README

To regenerate after TUI changes (requires [VHS](https://github.com/charmbracelet/vhs)):

```bash
vhs demo/knuckle-demo.tape
```

The GIF and cast are committed so contributors can preview them without re-running VHS.

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
