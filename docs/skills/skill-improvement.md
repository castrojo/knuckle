---
name: skill-improvement
description: "The skill-improvement mandate for this repo. Every session produces work + a skill file update. Use when completing a task and deciding whether a skill update is needed."
metadata:
  type: procedure
---

# Skill Improvement Mandate

Every agent session produces two outputs:

1. **The work** — the PR, fix, or feature
2. **The learning** — what a future agent needs to know

Output 1 without Output 2 leaves the factory no smarter.

## Before Marking Work Done

- [ ] Discovered a workaround, non-obvious pattern, or convention?
- [ ] Is there a skill file for the area worked in?
- [ ] If yes — updated it?
- [ ] If no — created one?
- [ ] Skill file committed in **this same PR**?

## What Counts

Write it: upstream bug workarounds, non-obvious correctness requirements, trial-and-error discoveries, common failure modes.

Do NOT write it: one-off task notes, ephemeral state, session logs, things obvious to any developer.

## Where

All learnings → `docs/skills/` in this repo. Cross-cutting patterns affecting 2+ repos → open issue in `projectbluefin/common` with `kind/improvement` + `area/agent`.

## What Is Banned

- No changelog files (`IMPROVEMENTS.md`, `CHANGELOG.md` for agent notes, `SESSION.md`, etc.). Delete them if found.
- No session notes committed to the repo.
- No "append here" instructions. Route to a specific `docs/skills/<file>.md`.

Full mandate: [`projectbluefin/common/docs/skills/skill-improvement.md`](https://github.com/projectbluefin/common/blob/main/docs/skills/skill-improvement.md)
