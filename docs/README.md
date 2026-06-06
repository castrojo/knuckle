# docs/ — Index

This directory contains supplementary documentation for knuckle. Start with the user-facing reference docs, then dip into the maintainer material if you are working on CI, release engineering, hardware labs, or design history.

## User-facing reference docs

| File | Description |
|------|-------------|
| [HEADLESS-CONFIG.md](HEADLESS-CONFIG.md) | Canonical reference for the `--headless --config` JSON schema, including fields, defaults, and validation rules. |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Practical runbook for common install failures, first-boot issues, and quick diagnostics. |
| [SECURITY.md](SECURITY.md) | Security posture, threat model, known gaps, and the disclosure path for vulnerabilities. |
| [CI-AND-TESTING.md](CI-AND-TESTING.md) | CI pipeline overview, test pyramid, coverage gates, and ISO build internals. |
| [SYSEXTS.md](SYSEXTS.md) | Reference guide to the Flatcar Bakery sysext catalog, support tiers, and extension behavior. |
| [BUTANE-DEPENDENCY.md](BUTANE-DEPENDENCY.md) | Explains why knuckle embeds Butane as a Go library instead of relying on the `butane` CLI. |
| [HIVE-DEPLOYMENT.md](HIVE-DEPLOYMENT.md) | Host-side deployment notes for running knuckle's Hive container on Flatcar. |
| [TEST-PLAN.md](TEST-PLAN.md) | High-level test plan covering expected behavior, edge cases, and coverage goals. |

## Internal / maintainer docs

These files are mainly useful for maintainers, reviewers, and agent-assisted development workflows.

| File | Description |
|------|-------------|
| [GHOST-LAB.md](GHOST-LAB.md) | Hardware lab notes for real-device and ghost test environment setup. |
| [PR-TEST-MATRIX.md](PR-TEST-MATRIX.md) | Maintainer-oriented matrix for mapping change types to the expected PR test coverage. |
| [RELEASE.md](RELEASE.md) | Release tag checklist — gates, VM verification, and tag procedure. |
| [SUBIQUITY-TUI-ANALYSIS.md](SUBIQUITY-TUI-ANALYSIS.md) | Design research comparing knuckle's installer UX to Ubuntu Subiquity. |
| [TUI-WIZARD-PATTERNS.md](TUI-WIZARD-PATTERNS.md) | Background research on wizard and TUI interaction patterns used to shape the current flow. |
| [internal/REVIEW-2026-05-19.md](internal/REVIEW-2026-05-19.md) | Agent-authored principal-engineer review notes from 2026-05-19. |
| [internal/REVIEW-2026-05-19b.md](internal/REVIEW-2026-05-19b.md) | Agent-authored follow-up review notes from 2026-05-19. |
| [internal/REVIEW-2026-05-20.md](internal/REVIEW-2026-05-20.md) | Agent-authored principal-engineer review notes from 2026-05-20. |

## Demo assets

The top-level `demo/` directory contains the recorded installer demo used in the project docs:

- `demo/knuckle-demo.tape` — [VHS](https://github.com/charmbracelet/vhs) script/source
- `demo/knuckle-install.cast` — generated [asciinema](https://asciinema.org/) cast
- `demo/knuckle-install.gif` — rendered GIF artifact

Regenerate the cast and GIF with:

```bash
vhs demo/knuckle-demo.tape
```
