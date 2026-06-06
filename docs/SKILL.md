# knuckle — Agent Skill Router

Agent entry point. Read this file first, then load the skill that matches your task.

## Task → Skill

| I need to… | Load |
|---|---|
| Review a PR or run vm-e2e tests | [`docs/skills/qa.md`](skills/qa.md) |
| Cut a release | [`docs/skills/release.md`](skills/release.md) |
| Run the VM locally or test an ISO | [`docs/skills/testlab.md`](skills/testlab.md) |
| Debug CI, understand coverage gates, or fix a failing check | [`docs/skills/ci.md`](skills/ci.md) |

## Reference docs (load on demand)

| Topic | File |
|---|---|
| Test pyramid, coverage thresholds, CI pipeline internals | [`CI-AND-TESTING.md`](CI-AND-TESTING.md) |
| PR test matrix, tier evidence standards, domain assertions | [`PR-TEST-MATRIX.md`](PR-TEST-MATRIX.md) |
| Release checklist, VM verification, blockers history | [`RELEASE.md`](RELEASE.md) |
| Headless config schema, field reference, validation rules | [`HEADLESS-CONFIG.md`](HEADLESS-CONFIG.md) |
| Security posture, threat model, disclosure path | [`SECURITY.md`](SECURITY.md) |
| Sysext catalog, Bakery support tiers, extension behavior | [`SYSEXTS.md`](SYSEXTS.md) |
| Troubleshooting runbook, first-boot diagnostics | [`TROUBLESHOOTING.md`](TROUBLESHOOTING.md) |
| Butane-as-library rationale | [`BUTANE-DEPENDENCY.md`](BUTANE-DEPENDENCY.md) |

## Scope rules

- **CI tasks** — touch only `.github/` and `docs/skills/ci.md`. Do not touch unrelated files.
- **PR review / QA** — follow `docs/skills/qa.md` exactly. One PR comment max; never duplicate GitHub UI state.
- **Release** — run the full E2E gate in `docs/skills/release.md` before tagging. No shortcuts.
- **Onboarding / doc tasks** — modify only `docs/` and `AGENTS.md`. No `.github/` changes unless the task is explicitly CI work.
- **The Justfile and `docs/` are the source of truth.** When memory and code disagree, code wins.

## Mandatory skill contribution

When you discover a new pattern, recurring mistake, or hard-won lesson, add it to the
relevant `docs/skills/*.md` file in the same PR as your change. All agents improve this
knowledge base — it is not read-only.
