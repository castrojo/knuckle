<!--
Thanks for sending a PR to knuckle.

Contributors using AI agents (Claude Code, Copilot, Cursor, pi):
AGENTS.md at the repo root is the authoritative agent context for
this project. Read it before writing code, and have the agent
verify it ran `just ci` locally before opening the PR. The hive
workflow assumes agents have respected the package boundaries and
safety invariants documented there.
-->

## What
<!-- One or two sentences. What does this change? -->

## Why
<!-- Link the issue this closes (Closes #123) or describe the motivation. -->

## How tested
<!-- Check at least one. -->
- [ ] `just ci` is green locally
- [ ] `just vm-e2e` passes (end-to-end install + boot)
- [ ] `just headless-test` passes (for headless-mode changes)
- [ ] Tested on real hardware — describe below
- [ ] Doc / template / CI change only — no install path touched

## Scope check
<!-- knuckle v1 supports single-disk, DHCP/static-IPv4, x86_64+arm64.
     Does this PR stay inside that scope? If it expands it, call out why. -->

## Notes for reviewers
<!-- Anything reviewers should know — risky areas, follow-ups, screenshots, etc. -->

---

<!-- For agents: mention what command you ran to verify, e.g.
     `just ci` exit 0, `just headless-test` exit 0. -->
