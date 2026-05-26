# docs/ — Index

This directory contains supplementary documentation for knuckle. Files are organized by audience.

## User-facing

| File | Description |
|------|-------------|
| [HEADLESS-CONFIG.md](HEADLESS-CONFIG.md) | **Authoritative reference** for the `--headless --config` JSON schema. Every field, type, default, and validation rule. Start here for automated/CI installs. |
| [SECURITY.md](SECURITY.md) | Threat model, supply-chain verification claims, known gaps, secret handling, and disclosure policy. |
| [SYSEXTS.md](SYSEXTS.md) | System extensions from the Flatcar Bakery: catalog structure, how sysexts are fetched and selected, architecture handling. |
| [GHOST-LAB.md](GHOST-LAB.md) | Hardware lab setup for real bare-metal testing. Primarily useful for maintainers with physical hardware. |

## Contributor-facing

| File | Description |
|------|-------------|
| [CI-AND-TESTING.md](CI-AND-TESTING.md) | Test pyramid, coverage gates, CI workflow overview, and how to run each layer locally. Read before adding tests. |
| [TEST-PLAN.md](TEST-PLAN.md) | Structured test plan covering TUI steps, headless paths, hardware edge cases, and sysext scenarios. |
| [PR-TEST-MATRIX.md](PR-TEST-MATRIX.md) | Detailed per-PR test matrix mapping change types to required test coverage. |
| [HIVE-DEPLOYMENT.md](HIVE-DEPLOYMENT.md) | Hive agent deployment context. Relevant if you are working on or with the AI agent pipeline. |
| [BUTANE-DEPENDENCY.md](BUTANE-DEPENDENCY.md) | Notes on the in-process Butane dependency, versioning, and why it is vendored rather than called as a CLI. |

## Internal / research

| File | Description | Maintained by |
|------|-------------|---|
| [TUI-WIZARD-PATTERNS.md](TUI-WIZARD-PATTERNS.md) | Internal research on TUI wizard UX patterns from analogous projects. Background for the current wizard design. | Human |
| [SUBIQUITY-TUI-ANALYSIS.md](SUBIQUITY-TUI-ANALYSIS.md) | Internal analysis of the Ubuntu Subiquity installer TUI. Historical design reference. | Human |
| [internal/REVIEW-2026-05-19.md](internal/REVIEW-2026-05-19.md) | Principal-engineer review notes — 2026-05-19. | Human (guide agent pass) |
| [internal/REVIEW-2026-05-19b.md](internal/REVIEW-2026-05-19b.md) | Principal-engineer review notes — 2026-05-19 (follow-up). | Human (guide agent pass) |
| [internal/REVIEW-2026-05-20.md](internal/REVIEW-2026-05-20.md) | Principal-engineer review notes — 2026-05-20. | Human (guide agent pass) |
