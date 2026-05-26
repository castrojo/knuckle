# Copilot Instructions — knuckle

> Full agent context lives in `AGENTS.md`. This file captures hard-learned operational
> rules that agents violate without explicit instruction.

## Build / Test

```bash
just ci           # full gate: tidy + fmt + vet + lint + vuln + test-race + cover-check + build
just build        # bin/knuckle (GOOS=linux GOARCH=amd64 CGO_ENABLED=0)
just test         # go test ./...
just cover-check  # per-package coverage threshold gate
just headless-test # config-gen e2e (runs on host, no VM)
```

`just ci` must be green before any commit or PR. Never `git push --no-verify`.

## Hard Rules

### ⛔ Never use kubectl via SSH to ghost

All KubeVirt / cluster operations go through **MCP tools** (`kubectl MCP`, `Argo MCP`) or
`just` recipes from `castrojo/testing-lab`. Never `ssh jorge@192.168.1.102 "kubectl ..."`.
This rule comes from `castrojo/testing-lab` AGENTS.md and RUNBOOK.md and applies here too.

### ⛔ Lab reports must be posted to the PR before merging

The workflow is strictly: **run qa-test-pr.sh → post report as PR comment → then merge**.
Never call `gh pr merge` before the strike report comment is on the PR. The report IS the
review evidence. No exceptions.

```bash
# Correct sequence:
gh pr comment <N> --repo projectbluefin/knuckle --body-file /tmp/qa-stdout-<N>.txt
gh pr review  <N> --repo projectbluefin/knuckle --approve        # or --request-changes
gh pr merge --auto <N> --repo projectbluefin/knuckle
```

### ⛔ projectbluefin/knuckle is a public repo — keep the homelab out of it

Infrastructure failures, ghost lab issues, and KubeVirt / QA pipeline bugs belong in
**`castrojo/testing-lab`** (private). Never file issues in `projectbluefin/knuckle` for:
- Ghost machine setup (missing remotes, auth, ports)
- KubeVirt VM boot timeouts
- QA script infrastructure bugs

`_file_issue_on_fail()` in `scripts/qa-test-pr.sh` writes a local `issue-body.md` only —
it does **not** auto-file anywhere. File lab failures manually in `castrojo/testing-lab`.

## Known Infrastructure Quirks

### `open /dev/tty: no such device or address` in ghost worktrees

`TestMain_TUINormalMode` and related `cmd/knuckle` TUI tests require a PTY. They fail under
`nohup`/non-interactive `ssh -f` execution on ghost. **This is a known infrastructure
limitation, not a PR regression.** When this appears:

- ✅ Rely on GitHub Actions CI as authoritative for `cmd/knuckle` coverage
- ✅ Note the limitation in the strike report and proceed
- ❌ Do not block or re-run the PR trying to fix a ghost PTY issue

Tracked in: projectbluefin/knuckle#512

### QA scripts: run sequentially, never in parallel

`scripts/qa-test-pr.sh` must run **one at a time**. Parallel runs race on `go.mod`/`go.sum`
(`"existing contents have changed since last read"`) and `golangci-lint` shared file lock.
Max concurrency: **1**.
