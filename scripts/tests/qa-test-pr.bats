#!/usr/bin/env bats
# Tests for scripts/qa-test-pr.sh argument validation, complexity gate, and tier routing.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/qa-test-pr.bats
#
# Strategy: qa-test-pr.sh is a monolithic script that depends on gh, git, ssh,
# jq, and a running KubeVirt cluster. We test pure-logic paths by:
# 1. Mocking external commands via PATH manipulation
# 2. Testing argument validation (missing PR number)
# 3. Testing the complexity gate with mock gh output
# 4. Testing tier routing by varying LABELS

SCRIPT="$BATS_TEST_DIRNAME/../qa-test-pr.sh"
MOCK_DIR=""

setup() {
  MOCK_DIR="$(mktemp -d)"
  export PATH="$MOCK_DIR:$PATH"
  export HOME="$MOCK_DIR"
  export MOCK_GIT_LOG="$MOCK_DIR/git.log"
  : > "$MOCK_GIT_LOG"
  mkdir -p "$MOCK_DIR/.ssh"
  touch "$MOCK_DIR/.ssh/id_ed25519"

  # Mock gh: returns configurable PR JSON
  cat > "$MOCK_DIR/gh" << 'GHEOF'
#!/usr/bin/env bash
case "$*" in
  *"pr view"*)
    echo "${MOCK_GH_PR_JSON:-{}}"
    ;;
  *"pr diff"*)
    echo "${MOCK_GH_DIFF_FILES:-}"
    ;;
  *)
    echo "mock-gh: $*" >&2
    ;;
esac
GHEOF
  chmod +x "$MOCK_DIR/gh"

  # Mock git: fake fetch/rev-parse/worktree and capture cleanup calls
  cat > "$MOCK_DIR/git" << 'GITEOF'
#!/usr/bin/env bash
[[ -n "${MOCK_GIT_LOG:-}" ]] && printf '%s\n' "$*" >> "$MOCK_GIT_LOG"
case "$1" in
  fetch) exit 0 ;;
  rev-parse) echo "abcdef123456" ;;
  show-ref)
    [[ "${MOCK_GIT_SHOW_REF:-0}" == "1" ]] && exit 0
    exit 1
    ;;
  update-ref) exit 0 ;;
  worktree)
    case "$2" in
      list) printf '%s' "${MOCK_GIT_WORKTREE_PORCELAIN:-}" ;;
      remove|add|prune) exit 0 ;;
      *) exit 0 ;;
    esac
    ;;
  branch)
    if [[ "$2" == "-D" && "${MOCK_GIT_BRANCH_DELETE_FAIL:-0}" == "1" ]]; then
      exit 1
    fi
    exit 0
    ;;
  *) exit 0 ;;
esac
GITEOF
  chmod +x "$MOCK_DIR/git"

  # Mock ssh: return fake OS version
  cat > "$MOCK_DIR/ssh" << 'SSHEOF'
#!/usr/bin/env bash
echo "3815.2.5"
SSHEOF
  chmod +x "$MOCK_DIR/ssh"

  # Mock scp
  cat > "$MOCK_DIR/scp" << 'SCPEOF'
#!/usr/bin/env bash
exit 0
SCPEOF
  chmod +x "$MOCK_DIR/scp"

  # Mock just: succeed immediately
  cat > "$MOCK_DIR/just" << 'JUSTEOF'
#!/usr/bin/env bash
echo "ok"
JUSTEOF
  chmod +x "$MOCK_DIR/just"

  # Mock jq: handle the common -r flag cases
  cat > "$MOCK_DIR/jq" << 'JQEOF'
#!/usr/bin/env bash
# Simple jq mock that parses the filter to return test values
input=$(cat)
filter="$2"
case "$filter" in
  .title)        echo "${MOCK_PR_TITLE:-test PR}" ;;
  .headRefName)  echo "${MOCK_PR_BRANCH:-feat/test}" ;;
  .author.login) echo "testuser" ;;
  *labels*name*join*) echo "${MOCK_PR_LABELS:-}" ;;
  *labels*size*) echo "${MOCK_PR_SIZE:-}" ;;
  *.body*Closes*) echo "" ;;
  *) echo "" ;;
esac
JQEOF
  chmod +x "$MOCK_DIR/jq"

  # Mock python3 (for swap config injection)
  cat > "$MOCK_DIR/python3" << 'PYEOF'
#!/usr/bin/env bash
exit 0
PYEOF
  chmod +x "$MOCK_DIR/python3"
}

teardown() {
  rm -rf "$MOCK_DIR"
}

# ── Argument validation ──────────────────────────────────────────────────────

@test "missing PR number exits with usage error" {
  run bash "$SCRIPT" 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"usage"* ]] || [[ "$output" == *"PR_NUMBER"* ]] || [[ "$output" == *"parameter"* ]]
}

# ── Complexity gate ──────────────────────────────────────────────────────────

@test "stale PR ref and worktree are cleaned before fetch" {
  export MOCK_GH_PR_JSON='{"title":"heal","headRefName":"feat/heal","labels":[{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="heal"
  export MOCK_PR_BRANCH="feat/heal"
  export MOCK_PR_LABELS="size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  export MOCK_GIT_SHOW_REF=1
  export MOCK_GIT_WORKTREE_PORCELAIN=$'worktree /tmp/knuckle-qa-wt-999\nHEAD deadbeef\nbranch refs/heads/pr999-qa\n\n'

  run bash "$SCRIPT" 999 2>&1

  [ -f "$MOCK_GIT_LOG" ]
  run grep -F "worktree prune" "$MOCK_GIT_LOG"
  [ "$status" -eq 0 ]
  run grep -F "worktree remove --force /tmp/knuckle-qa-wt-999" "$MOCK_GIT_LOG"
  [ "$status" -eq 0 ]
  run grep -F "branch -D pr999-qa" "$MOCK_GIT_LOG"
  [ "$status" -eq 0 ]
  run grep -F "fetch upstream +pull/999/head:refs/heads/pr999-qa -q" "$MOCK_GIT_LOG"
  [ "$status" -eq 0 ]
}

@test "size:XL triggers complexity gate (exit 2)" {
  export MOCK_GH_PR_JSON='{"title":"big PR","headRefName":"feat/big","labels":[{"name":"size:XL"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="big PR"
  export MOCK_PR_BRANCH="feat/big"
  export MOCK_PR_LABELS="size:XL"
  export MOCK_PR_SIZE="size:XL"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [ "$status" -eq 2 ]
  [[ "$output" == *"NOGO"* ]] || [[ "$output" == *"Complexity"* ]] || [[ "$output" == *"complexity"* ]]
}

@test "size:XXL triggers complexity gate (exit 2)" {
  export MOCK_GH_PR_JSON='{"title":"huge PR","headRefName":"feat/huge","labels":[{"name":"size:XXL"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="huge PR"
  export MOCK_PR_BRANCH="feat/huge"
  export MOCK_PR_LABELS="size:XXL"
  export MOCK_PR_SIZE="size:XXL"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [ "$status" -eq 2 ]
  [[ "$output" == *"NOGO"* ]] || [[ "$output" == *"Complexity"* ]] || [[ "$output" == *"complexity"* ]]
}

@test ">4 domain labels triggers complexity gate (exit 2)" {
  export MOCK_GH_PR_JSON='{"title":"multi","headRefName":"feat/m","labels":[{"name":"domain:a"},{"name":"domain:b"},{"name":"domain:c"},{"name":"domain:d"},{"name":"domain:e"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="multi"
  export MOCK_PR_BRANCH="feat/m"
  export MOCK_PR_LABELS="domain:a, domain:b, domain:c, domain:d, domain:e"
  export MOCK_PR_SIZE=""
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [ "$status" -eq 2 ]
  [[ "$output" == *"NOGO"* ]] || [[ "$output" == *"Complexity"* ]] || [[ "$output" == *"complexity"* ]]
}

@test "workflow file changes trigger complexity gate (exit 2)" {
  export MOCK_GH_PR_JSON='{"title":"wf","headRefName":"feat/wf","labels":[],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="wf"
  export MOCK_PR_BRANCH="feat/wf"
  export MOCK_PR_LABELS=""
  export MOCK_PR_SIZE=""
  export MOCK_GH_DIFF_FILES=".github/workflows/ci.yml"
  run bash "$SCRIPT" 999 2>&1
  [ "$status" -eq 2 ]
  [[ "$output" == *"NOGO"* ]] || [[ "$output" == *"Complexity"* ]] || [[ "$output" == *"complexity"* ]]
}

@test "size:M does NOT trigger complexity gate" {
  export MOCK_GH_PR_JSON='{"title":"small","headRefName":"feat/sm","labels":[{"name":"size:M"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="small"
  export MOCK_PR_BRANCH="feat/sm"
  export MOCK_PR_LABELS="size:M"
  export MOCK_PR_SIZE="size:M"
  export MOCK_GH_DIFF_FILES=""
  # Should NOT exit 2 (may fail later due to mock limitations, but not at complexity gate)
  run bash "$SCRIPT" 999 2>&1
  [ "$status" -ne 2 ]
}

# ── Tier routing ─────────────────────────────────────────────────────────────

@test "domain:probe routes to Tier 1" {
  export MOCK_GH_PR_JSON='{"title":"probe","headRefName":"feat/p","labels":[{"name":"domain:probe"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="probe"
  export MOCK_PR_BRANCH="feat/p"
  export MOCK_PR_LABELS="domain:probe, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=1"* ]]
}

@test "domain:tui routes to Tier 1" {
  export MOCK_GH_PR_JSON='{"title":"tui","headRefName":"feat/t","labels":[{"name":"domain:tui"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="tui"
  export MOCK_PR_BRANCH="feat/t"
  export MOCK_PR_LABELS="domain:tui, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=1"* ]]
}

@test "domain:install routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"inst","headRefName":"feat/i","labels":[{"name":"domain:install"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="inst"
  export MOCK_PR_BRANCH="feat/i"
  export MOCK_PR_LABELS="domain:install, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "domain:headless routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"hl","headRefName":"feat/h","labels":[{"name":"domain:headless"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="hl"
  export MOCK_PR_BRANCH="feat/h"
  export MOCK_PR_LABELS="domain:headless, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "domain:ignition routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"ign","headRefName":"feat/ig","labels":[{"name":"domain:ignition"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="ign"
  export MOCK_PR_BRANCH="feat/ig"
  export MOCK_PR_LABELS="domain:ignition, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "domain:sysext routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"sx","headRefName":"feat/sx","labels":[{"name":"domain:sysext"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="sx"
  export MOCK_PR_BRANCH="feat/sx"
  export MOCK_PR_LABELS="domain:sysext, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "swap label routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"sw","headRefName":"feat/sw","labels":[{"name":"swap"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="sw"
  export MOCK_PR_BRANCH="feat/sw"
  export MOCK_PR_LABELS="swap, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "tailscale label routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"ts","headRefName":"feat/ts","labels":[{"name":"tailscale"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="ts"
  export MOCK_PR_BRANCH="feat/ts"
  export MOCK_PR_LABELS="tailscale, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "domain:security routes to Tier 1 with DO_SECURITY=1" {
  export MOCK_GH_PR_JSON='{"title":"sec","headRefName":"feat/sec","labels":[{"name":"domain:security"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="sec"
  export MOCK_PR_BRANCH="feat/sec"
  export MOCK_PR_LABELS="domain:security, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=1"* ]]
  [[ "$output" == *"security=1"* ]]
}

@test "no domain labels stays at Tier 0" {
  export MOCK_GH_PR_JSON='{"title":"doc","headRefName":"docs/readme","labels":[{"name":"size:S"},{"name":"docs"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="doc"
  export MOCK_PR_BRANCH="docs/readme"
  export MOCK_PR_LABELS="size:S, docs"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=0"* ]]
  [[ "$output" == *"needs_boot=0"* ]]
}

# ── Worktree self-heal (commit b16d36a) ──────────────────────────────────────

@test "stale branch is deleted before fetch" {
  # Replace git mock with one that logs invocations
  GIT_LOG="$MOCK_DIR/git_invocations.log"
  cat > "$MOCK_DIR/git" << 'GITEOF'
#!/usr/bin/env bash
echo "$*" >> "${GIT_LOG:-/dev/null}"
case "$1" in
  branch)   exit 0 ;;
  fetch)    exit 0 ;;
  rev-parse) echo "abcdef123456" ;;
  worktree) exit 0 ;;
  *)        exit 0 ;;
esac
GITEOF
  chmod +x "$MOCK_DIR/git"
  export GIT_LOG

  export MOCK_GH_PR_JSON='{"title":"doc","headRefName":"docs/readme","labels":[{"name":"size:S"},{"name":"docs"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="doc"
  export MOCK_PR_BRANCH="docs/readme"
  export MOCK_PR_LABELS="size:S, docs"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""

  run bash "$SCRIPT" 999 2>&1

  # Verify branch -D is called before fetch
  [ -f "$GIT_LOG" ]
  grep -q "branch -D pr999-qa" "$GIT_LOG"
  # branch -D must appear before fetch
  branch_line=$(grep -n "branch -D" "$GIT_LOG" | head -1 | cut -d: -f1)
  fetch_line=$(grep -n "fetch upstream" "$GIT_LOG" | head -1 | cut -d: -f1)
  [ "$branch_line" -lt "$fetch_line" ]
}

@test "stale worktree directory triggers cleanup before add" {
  # Pre-create the worktree path to trigger the self-heal branch
  WORKTREE_PATH="/tmp/knuckle-qa-wt-999"
  mkdir -p "$WORKTREE_PATH"

  GIT_LOG="$MOCK_DIR/git_invocations.log"
  cat > "$MOCK_DIR/git" << 'GITEOF'
#!/usr/bin/env bash
echo "$*" >> "${GIT_LOG:-/dev/null}"
case "$1" in
  branch)   exit 0 ;;
  fetch)    exit 0 ;;
  rev-parse) echo "abcdef123456" ;;
  worktree)
    if [[ "$2" == "remove" ]]; then
      # Simulate successful removal
      rm -rf "$3" 2>/dev/null || true
      exit 0
    fi
    exit 0
    ;;
  *)        exit 0 ;;
esac
GITEOF
  chmod +x "$MOCK_DIR/git"
  export GIT_LOG

  export MOCK_GH_PR_JSON='{"title":"doc","headRefName":"docs/readme","labels":[{"name":"size:S"},{"name":"docs"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="doc"
  export MOCK_PR_BRANCH="docs/readme"
  export MOCK_PR_LABELS="size:S, docs"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""

  run bash "$SCRIPT" 999 2>&1

  # Verify worktree remove --force was invoked
  [ -f "$GIT_LOG" ]
  grep -q "worktree remove --force /tmp/knuckle-qa-wt-999" "$GIT_LOG"
  # worktree remove must appear before worktree add
  remove_line=$(grep -n "worktree remove" "$GIT_LOG" | head -1 | cut -d: -f1)
  add_line=$(grep -n "worktree add" "$GIT_LOG" | head -1 | cut -d: -f1)
  [ "$remove_line" -lt "$add_line" ]

  # Cleanup
  rm -rf "$WORKTREE_PATH" 2>/dev/null || true
}

@test "worktree fallback to rm -rf when git worktree remove fails" {
  # Pre-create the worktree path
  WORKTREE_PATH="/tmp/knuckle-qa-wt-999"
  mkdir -p "$WORKTREE_PATH"
  touch "$WORKTREE_PATH/sentinel"

  GIT_LOG="$MOCK_DIR/git_invocations.log"
  cat > "$MOCK_DIR/git" << 'GITEOF'
#!/usr/bin/env bash
echo "$*" >> "${GIT_LOG:-/dev/null}"
case "$1" in
  branch)   exit 0 ;;
  fetch)    exit 0 ;;
  rev-parse) echo "abcdef123456" ;;
  worktree)
    if [[ "$2" == "remove" ]]; then
      # Simulate failure — rm -rf fallback should clean up
      exit 1
    fi
    exit 0
    ;;
  *)        exit 0 ;;
esac
GITEOF
  chmod +x "$MOCK_DIR/git"
  export GIT_LOG

  export MOCK_GH_PR_JSON='{"title":"doc","headRefName":"docs/readme","labels":[{"name":"size:S"},{"name":"docs"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="doc"
  export MOCK_PR_BRANCH="docs/readme"
  export MOCK_PR_LABELS="size:S, docs"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""

  run bash "$SCRIPT" 999 2>&1

  # The rm -rf fallback should have removed the directory
  [ ! -d "$WORKTREE_PATH" ]

  # Cleanup (in case test logic changes)
  rm -rf "$WORKTREE_PATH" 2>/dev/null || true
}

# ── domain:iso tier routing (issue #672) ─────────────────────────────────────

@test "domain:iso routes to Tier 3 with needs_boot" {
  export MOCK_GH_PR_JSON='{"title":"iso","headRefName":"feat/iso","labels":[{"name":"domain:iso"},{"name":"size:S"}],"body":"","author":{"login":"dev"}}'
  export MOCK_PR_TITLE="iso"
  export MOCK_PR_BRANCH="feat/iso"
  export MOCK_PR_LABELS="domain:iso, size:S"
  export MOCK_PR_SIZE="size:S"
  export MOCK_GH_DIFF_FILES=""
  run bash "$SCRIPT" 999 2>&1
  [[ "$output" == *"tier=3"* ]]
  [[ "$output" == *"needs_boot=1"* ]]
}

@test "non-routing domain labels (bakery, wizard, validate, ci) stay at Tier 0" {
  for label in domain:bakery domain:wizard domain:validate domain:ci; do
    export MOCK_GH_PR_JSON="{\"title\":\"x\",\"headRefName\":\"feat/x\",\"labels\":[{\"name\":\"${label}\"},{\"name\":\"size:S\"}],\"body\":\"\",\"author\":{\"login\":\"dev\"}}"
    export MOCK_PR_TITLE="x"
    export MOCK_PR_BRANCH="feat/x"
    export MOCK_PR_LABELS="${label}, size:S"
    export MOCK_PR_SIZE="size:S"
    export MOCK_GH_DIFF_FILES=""
    run bash "$SCRIPT" 999 2>&1
    [[ "$output" != *"tier=3"* ]] || { echo "FAIL: ${label} should not route to tier=3"; exit 1; }
    [[ "$output" != *"tier=1"* ]] || { echo "FAIL: ${label} should not route to tier=1"; exit 1; }
  done
}
