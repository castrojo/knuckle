# QA — knuckle PR Review + VM E2E

End-to-end PR review: preflight → tier classification → code review → vm-e2e → decision.

> Reference docs: [`PR-TEST-MATRIX.md`](../PR-TEST-MATRIX.md), [`CI-AND-TESTING.md`](../CI-AND-TESTING.md)

## Pre-Flight (once per session)

```bash
# Check for first-time contributor CI holds
gh api repos/projectbluefin/knuckle/actions/runs?status=action_required \
  --jq '.workflow_runs[] | "\(.id) \(.name) \(.head_branch)"'
# Unblock each: gh api repos/projectbluefin/knuckle/actions/runs/<ID>/approve --method POST

# List open PRs with sizes and labels
gh pr list --repo projectbluefin/knuckle --state open \
  --json number,title,labels,additions,deletions \
  --jq '.[] | "#\(.number) \(.additions+.deletions)L [\(.labels|map(.name)|join(","))] \(.title)"'
```

⛔ **Run QA scripts SEQUENTIALLY, not in parallel.** Parallel `go mod tidy` runs
race on `go.mod`/`go.sum`. Parallel `golangci-lint` races on its file lock.

## Tier Classification

Tier is set by the **highest-tier domain label** on the PR. Labels only — never PR title.

| Labels | Tier | What runs |
|---|---|---|
| `domain:ci`, `kind/test`, docs-only | 0 | `just ci` on dev machine |
| `domain:probe`, `domain:tui`, `domain:validate` | 1 | Tier 0 + tool check + dry-run |
| `domain:security` | 1+sec | Tier 1 + bad-input rejection tests |
| `domain:install`, `domain:headless`, `domain:ignition`, swap, tailscale, sysext | **3** | Tier 1 + full headless install + boot + domain assertions |
| `domain:iso` | 3 | Tier 3 + hardware-repro |

## Complexity Gate (skip vm-e2e if ANY trigger)

| Signal | Threshold |
|---|---|
| `size:XL` or `size:XXL` label | present |
| Domain labels | > 4 distinct `domain:*` |
| Workflow files changed | any `.github/workflows/*.yml` |
| Architecture boundary | `cmd/knuckle` + `internal/runner` + `internal/ignition` together |

On complexity gate: code review only, no vm-e2e, leave review without queuing.

## Code Review Checklist

```
□ gofmt clean (double space before // is the most common failure)
□ No exec.Command outside internal/runner
□ Disk identity via /dev/disk/by-id (not /dev/sdX)
□ Ignition tempfile: os.CreateTemp + chmod 0600 + defer os.Remove
□ No secrets in slog output
□ Test assertions check err.Error() content, not just err != nil
□ Permission tests: t.Skip if os.Getuid() == 0
□ Every LGTM backed by a file:line reference from the diff
□ SA5011: return immediately after every t.Fatal nil-check guard
```

### Domain-specific checks

| Domain | Key check |
|---|---|
| `install` | `wipefs → flatcar-install → sfdisk` order; DryRunner no-ops all three |
| `ignition` | `{{- end}}` balanced; `yamlEscape` on every user string |
| `headless` | `Validate()` called before `ToInstallConfig()`; SSH keys validated |
| `tui` | No business logic in view model; `wizard.Apply*` for mutations |
| `validate` | Table-driven tests; error messages include the bad value |
| `wizard` | Conditional steps check selector in Next/Previous/GoToStep |
| `bakery` | SHA512 + GPG both checked; no per-call `http.Client` |
| `ci/release` | `persist-credentials: false` on all checkout steps |

## VM E2E Test

### Option A — GitHub Actions (preferred for Tier 3)

```bash
# Trigger all 4 passes on the PR branch
gh workflow run vm-e2e.yml \
  --repo projectbluefin/knuckle \
  --ref <branch-name>

# Watch
gh run list --repo projectbluefin/knuckle --workflow vm-e2e.yml --limit 5
gh run view <RUN_ID> --repo projectbluefin/knuckle
```

> ⚠️ `workflow_dispatch` only works once `vm-e2e.yml` is on `main`. For branches with
> a pending merge of `vm-e2e.yml`, use Option B.

### Option B — Local

```bash
cd ~/src/knuckle
git checkout <pr-branch>
just vm-e2e   # 4 passes: DHCP → static → sysext → NVIDIA
```

Requires `/dev/kvm` + QEMU. Flatcar base image (~480 MB) cached at `.vm/flatcar_base_amd64.img`.

### 4 passes — what each verifies

| Pass | Key assertions |
|---|---|
| DHCP | hostname, update strategy, core user groups |
| Static | `/etc/systemd/network/10-static.network` (address, gateway, interface) |
| Sysext | `docker.raw` present + size, `systemd-sysext` active, `docker version` |
| NVIDIA | `/etc/flatcar/enabled-sysext.conf` contains `nvidia-drivers-*` |

**NVIDIA pass config:** use `"nvidia_driver_version":"570-open"` (flat string).
The nested `"nvidia":{"enabled":true}` field is silently ignored by Go JSON.

## Decision & Merge

```bash
# GO — post report, approve, queue
gh pr comment <N> --repo projectbluefin/knuckle --body-file /tmp/qa-report.txt
gh pr review <N> --repo projectbluefin/knuckle --approve
gh pr merge --auto <N> --repo projectbluefin/knuckle  # ALWAYS --auto

# NOGO — post report, request changes (gh requires non-empty body)
gh pr comment <N> --repo projectbluefin/knuckle --body-file /tmp/qa-report.txt
gh pr review <N> --repo projectbluefin/knuckle --request-changes \
  --body "See strike report comment for requested changes."
```

⛔ **Always `gh pr merge --auto`.** Direct merge bypasses CI on the combined branch.
⛔ **Cannot self-approve PRs** authored by the agent account.
⛔ **Workflow files (`.github/workflows/*.yml`)** cannot be auto-merged — Jorge merges via UI.

## PR Comment Rules

- **One comment per PR event, max.** Combine all findings. Never post a follow-up — edit instead.
- **Never duplicate GitHub UI state.** No approval counts, merge queue status, or CI pass/fail summaries.
- **Test reports: minimal.** What ran, pass/fail, blockers only.
- **When in doubt, don't post.** If the only thing to report is "tests pass", post nothing.

## Merge Conflicts

```bash
# Try GitHub auto-rebase first
gh pr update-branch <N> --repo projectbluefin/knuckle
# If that fails, cherry-pick locally:
git fetch upstream pull/<N>/head:pr<N>-head
git checkout -B fix/<N>-rebased upstream/main
git cherry-pick pr<N>-head
git push castrojo fix/<N>-rebased:<original-branch> --force-with-lease
```

Never regex-based conflict surgery on Go files. Require `go build ./...` before staging.

## Worktree Hygiene

Clean up stale worktrees at end of every batch session:

```bash
cd ~/src/knuckle
for wt in $(git worktree list --porcelain | grep worktree | awk '{print $2}' | grep /tmp/knuckle-pr-); do
  git worktree remove "$wt" --force 2>/dev/null && echo "removed $wt"
done
git worktree list
```

## Common Failures

| Failure | Cause | Fix |
|---|---|---|
| `open /dev/tty: no such device` (cmd/knuckle tests) | No PTY in non-interactive env | Pre-existing issue #512; rely on GHA CI (authoritative) |
| `go: updating go.mod: existing contents have changed` | Parallel QA races on go.mod | Run QA sequentially |
| PR stuck `BLOCKED`, no CI runs | First-time contributor | Approve via `gh api ... /actions/runs/<ID>/approve` |
| `git worktree add` fails for `/tmp/knuckle-qa-wt-<N>` | Stale worktree | `git worktree remove /tmp/knuckle-qa-wt-<N> --force` |
| `git fetch ... pr<N>-qa` exits 128 | Stale local ref | `git update-ref -d refs/heads/pr<N>-qa` then retry |

## Lessons Learned

### Sequential QA only (2026-05-24)

Parallel `go mod tidy` races on `go.mod`/`go.sum` ("existing contents have changed since
last read"). Parallel `golangci-lint` races on its file lock. **Max 1 QA script at a time.**
The "max 3 concurrent" rule written earlier was wrong.

### Feature injection uses LABELS, never PR title (2026-05-24)

A PR titled "fix tailscale tests" with only `domain:validate` labels is Tier 0, not
Tier 3. Check `$LABELS`, never `$TITLE`, to decide whether to inject tailscale auth key
into the QA config.

### Hanthor PRs: check the actual diff (2026-05-26)

Stale branches accumulate all upstream changes — `git diff merge-base..pr-HEAD --stat`
to isolate the actual change. `size:XXL` triggers the complexity gate; do not ghost-test.

### `vm-e2e.yml` NVIDIA pass config (2026-05-30)

NVIDIA pass config must use `"nvidia_driver_version":"570-open"` (flat string).
The nested `"nvidia":{"enabled":true,"driver_type":"open"}` is silently ignored by Go JSON.
