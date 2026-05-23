#!/usr/bin/env bash
# scripts/qa-test-pr.sh — Knuckle PR ghost testlab harness
#
# Invoked via: just qa-pr <PR_NUMBER>
#
# Build → unit tests → installer VM → headless install → boot installed
# Flatcar → domain assertions with quoted evidence → run artifacts saved.
#
# Exit codes: 0 = all pass | 1 = failure | 2 = complex PR (skip)

set -euo pipefail

PR=${1:?usage: qa-test-pr.sh <PR_NUMBER>}

# QA_HOST: set to user@hostname to run on a remote machine.
# Leave unset (or set to localhost) to run entirely on this machine.
# Example: export QA_HOST=jorge@192.168.1.102
# See docs/GHOST-LAB.md for lab setup instructions.
GHOST=${QA_HOST:-localhost}
GOPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o IdentitiesOnly=yes -i ${HOME}/.ssh/id_ed25519"
WORK_REMOTE="/var/tmp/knuckle-qa-pr-${PR}"
RUN_ID="pr-${PR}-$(date +%Y%m%d-%H%M%S)"
RUNDIR=".qa/runs/${RUN_ID}"
REPORT="${RUNDIR}/report.md"
START=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

_ghost()     { ssh $GOPTS "$GHOST" "$@"; }
_scp_to()    { scp $GOPTS "$@"; }
_scp_from()  { scp $GOPTS "$@"; }
log()        { echo "  [qa ${RUN_ID}] $*" >&2; }

_fetch_artifacts() {
  log "Fetching artifacts from ghost..."
  _scp_from $GOPTS -r "$GHOST:${WORK_REMOTE}/" "${RUNDIR}/ghost/" 2>/dev/null || true
}

_file_issue_on_fail() {
  local report="$1" rundir="$2" summary="$3"
  local issue_file="${rundir}/issue-body.md"
  cat > "$issue_file" << ISSUE_EOF
## QA Failure: PR #${PR} — ${TITLE}

**Summary:** ${summary}
**Run:** ${RUN_ID}
**Commit:** ${SHA}
**Flatcar:** ${FLATCAR_VER}
**Labels:** ${LABELS}

### Failing output

\`\`\`
$(grep -A3 "FAIL\|❌" "$report" | head -30)
\`\`\`

### To reproduce
\`\`\`bash
just qa-pr ${PR}
\`\`\`
ISSUE_EOF
  log "Issue body: ${issue_file}"
}

# Source KubeVirt helpers
# shellcheck source=scripts/lib/vm-kubevirt.sh
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/vm-kubevirt.sh"

VM_NAME=""  # set when VM is allocated; trap uses this
_cleanup_kv() {
  local exit_code=$?
  [[ -n "${VM_NAME:-}" ]] && kv_delete "${VM_NAME}" 2>/dev/null || true
  exit "$exit_code"
}
trap '_cleanup_kv' EXIT INT TERM

mkdir -p "$RUNDIR"

# ── 1. PR metadata ────────────────────────────────────────────────────────────
log "Fetching PR #${PR}..."
PR_JSON=$(gh pr view "$PR" --repo projectbluefin/knuckle \
  --json title,headRefName,labels,body,author 2>/dev/null)
TITLE=$(  echo "$PR_JSON" | jq -r '.title')
BRANCH=$( echo "$PR_JSON" | jq -r '.headRefName')
AUTHOR=$( echo "$PR_JSON" | jq -r '.author.login')
LABELS=$( echo "$PR_JSON" | jq -r '[.labels[].name] | join(", ")')
SIZE=$(   echo "$PR_JSON" | jq -r '[.labels[] | select(.name|startswith("size:")) | .name] | first // ""')
CLOSES=$( echo "$PR_JSON" | jq -r '.body' | grep -oP 'Closes #\K\d+' | tr '\n' ',' | sed 's/,$//' || echo "")
DOMAIN_COUNT=$(echo "$LABELS" | tr ',' '\n' | grep -c "domain:" || true)
WF_CHANGED=$(gh pr diff "$PR" --repo projectbluefin/knuckle --name-only 2>/dev/null \
  | grep -c "^\.github/workflows/" || true)

# ── 2. Complexity gate ────────────────────────────────────────────────────────
if [[ "$SIZE" == "size:XL" || "$SIZE" == "size:XXL" ]] || [[ $DOMAIN_COUNT -gt 4 ]] || [[ $WF_CHANGED -gt 0 ]]; then
  log "SKIP: complexity gate (size=${SIZE} domains=${DOMAIN_COUNT} wf=${WF_CHANGED})"
  cat > "$REPORT" << EOF
## 🧪 Ghost Testlab — PR #${PR} SKIPPED

**Reason:** Complexity gate — size: ${SIZE}, domains: ${DOMAIN_COUNT}, workflow changes: ${WF_CHANGED}
**Required:** Human \`just vm-e2e\` on ghost (192.168.1.102) before merge.
EOF
  cat "$REPORT"; exit 2
fi

# ── 3. What to test ───────────────────────────────────────────────────────────
# Tier 0: unit tests (dev machine only)
# Tier 1: installer VM + tool check + --dry-run + security bad-input tests
# Tier 3: Tier 1 + real headless install + BOOT installed Flatcar + assertions
#   (Tier 3 is the default for any PR touching the installed system)

TIER=0
NEEDS_BOOT=0
DO_SECURITY=0

_has() { echo "$LABELS" | grep -q "$1"; }

_has "domain:probe"      && TIER=1
_has "domain:tui"        && TIER=1
_has "domain:security"   && TIER=1 && DO_SECURITY=1
_has "domain:install"    && TIER=3 && NEEDS_BOOT=1
_has "domain:ignition"   && TIER=3 && NEEDS_BOOT=1
_has "domain:headless"   && TIER=3 && NEEDS_BOOT=1
_has "domain:sysext"     && TIER=3 && NEEDS_BOOT=1
echo "$TITLE $LABELS" | grep -qi "swap"      && TIER=3 && NEEDS_BOOT=1
echo "$TITLE $LABELS" | grep -qi "tailscale" && TIER=3 && NEEDS_BOOT=1

log "tier=${TIER} needs_boot=${NEEDS_BOOT} security=${DO_SECURITY}"

# ── 4. Build ──────────────────────────────────────────────────────────────────
PREV=$(git branch --show-current 2>/dev/null || echo "main")
git fetch upstream "pull/${PR}/head:pr${PR}-qa" -q 2>/dev/null
SHA=$(git rev-parse "pr${PR}-qa" | head -c 12)
git stash -q 2>/dev/null || true
git checkout "pr${PR}-qa" -q

log "Building ${SHA}..."
just build > "${RUNDIR}/build.log" 2>&1 && BUILD_OK=1 || BUILD_OK=0
git checkout "$PREV" -q 2>/dev/null || true
git stash pop -q 2>/dev/null || true

# ── 5. Report header ──────────────────────────────────────────────────────────
FLATCAR_VER=$(_ghost "grep -m1 VERSION_ID= /etc/os-release 2>/dev/null | cut -d= -f2" 2>/dev/null || echo "unknown")

cat > "$REPORT" << EOF
## 🧪 Ghost Testlab Report — PR #${PR}

| | |
|---|---|
| **PR** | ${TITLE} |
| **Author** | ${AUTHOR} |
| **Branch** | \`${BRANCH}\` @ \`${SHA}\` |
| **Closes** | ${CLOSES:-—} |
| **Flatcar** | ${FLATCAR_VER} |
| **Labels** | ${LABELS} |
| **Tier** | ${TIER} | boot verification: $([ $NEEDS_BOOT -eq 1 ] && echo "yes ✓" || echo "no") |
| **Run** | ${RUN_ID} |
| **Date** | ${START} |

---

EOF

if [[ $BUILD_OK -eq 0 ]]; then
  { echo "### Build"; echo '```'; cat "${RUNDIR}/build.log" | tail -20; echo '```'; echo; echo "**Verdict: ❌ BUILD FAILED**"; } >> "$REPORT"
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Build failed" || true
  cat "$REPORT"; exit 1
fi

# ── 6. Tier 0 — CI gate ───────────────────────────────────────────────────────
log "Tier 0: just ci..."
git checkout "pr${PR}-qa" -q
just ci > "${RUNDIR}/ci.log" 2>&1 && CI_OK=1 || CI_OK=0
git checkout "$PREV" -q 2>/dev/null || true

CI_SUMMARY=$(grep -E "^ok |^FAIL|✅|PASS|cover:|error:" "${RUNDIR}/ci.log" | tail -20)

{
  echo "### Tier 0 — CI gate (dev machine)"
  echo
  echo '```'
  echo "$CI_SUMMARY"
  echo '```'
  echo
  [[ $CI_OK -eq 1 ]] && echo "**✅ TIER 0 PASS**" || echo "**❌ TIER 0 FAIL**"
  echo
} >> "$REPORT"

if [[ $CI_OK -eq 0 ]]; then
  echo "**Verdict: ❌ FAIL — CI gate not green**" >> "$REPORT"
  _file_issue_on_fail "$REPORT" "$RUNDIR" "CI gate failed" || true
  cat "$REPORT"; exit 1
fi

[[ $TIER -eq 0 ]] && {
  echo "**Verdict: ✅ PASS** — unit tests sufficient for this label set" >> "$REPORT"
  log "Done (tier 0)"
  cat "$REPORT"; exit 0
}

# ── 7. Create KubeVirt installer VM ──────────────────────────────────────────
log "Setting up KubeVirt installer VM..."
VM_NAME="qa-pr-${PR}-$(date +%s)"

_ghost "mkdir -p ${WORK_REMOTE}"

HOST_KEY=$(_ghost "cat ~/.ssh/id_ed25519.pub")

log "Preparing disks..."
kv_prepare_disk "$VM_NAME"

log "Applying VM to cluster..."
kv_apply_vm "$VM_NAME"

log "Injecting SSH key..."
kv_inject_ssh_key "$VM_NAME"

log "Waiting for installer VM ready..."
kv_wait_ready "$VM_NAME" 120 || {
  { echo "### Installer VM Boot"; echo "**⛔ BOOT TIMEOUT**"; echo; echo "**Verdict: ❌ FAIL**"; } >> "$REPORT"
  cat "$REPORT"; exit 1
}

kv_wait_ssh "$VM_NAME" 120 || {
  { echo "### Installer VM Boot"; echo "**⛔ SSH TIMEOUT — Flatcar did not finish booting**"; echo; echo "**Verdict: ❌ FAIL**"; } >> "$REPORT"
  cat "$REPORT"; exit 1
}

INSTALLER_IP=$(kv_ip "$VM_NAME")
log "Installer VM ready at ${INSTALLER_IP}"

kv_scp_to_vm "$VM_NAME" bin/knuckle /tmp/knuckle
kv_ssh "$VM_NAME" 'chmod +x /tmp/knuckle'
log "Binary deployed"

# ── 8. Tier 1 — tool check + dry-run ─────────────────────────────────────────
log "Tier 1: tool check + dry-run..."

# Build the headless config — include feature-specific fields based on labels
# Write headless config to a local file and SCP through ghost into the VM.
# This avoids multi-layer shell quoting of JSON — heredoc expands once cleanly.
QA_CONFIG_FILE="/tmp/knuckle-qa-config-${PR}.json"

# Base config
cat > "${QA_CONFIG_FILE}" << JSONEOF
{"channel":"stable","hostname":"qa-pr-${PR}","timezone":"UTC","network":{"mode":"dhcp"},"users":[{"username":"core","ssh_keys":["${HOST_KEY}"]}],"disk":"/dev/vdb","update_strategy":"off","reboot":false}
JSONEOF

# Inject swap config using 512 MiB — small enough to fit in the VM /var partition.
# (Default 4 GiB exhausts the partition; 512 MiB is sufficient to verify the feature.)
if echo "$TITLE $LABELS" | grep -qi "swap"; then
  python3 - "${QA_CONFIG_FILE}" << 'PYEOF'
import json, sys
p = sys.argv[1]
d = json.load(open(p))
d["swap"] = {"enabled": True, "size_mb": 512}
json.dump(d, open(p, "w"))
PYEOF
fi

if echo "$TITLE $LABELS" | grep -qi "tailscale"; then
  python3 - "${QA_CONFIG_FILE}" << 'PYEOF'
import json, sys
p = sys.argv[1]
d = json.load(open(p))
d["tailscale"] = {"auth_key": "tskey-auth-abcdef1234567890AB-CDEFGHIJKLMNOPQRSTUVWXYZ0123456789"}
json.dump(d, open(p, "w"))
PYEOF
fi

# SCP config to ghost for artifact storage, then directly into the VM
_scp_to $GOPTS "${QA_CONFIG_FILE}" "$GHOST:${WORK_REMOTE}/qa.json"
kv_scp_to_vm "$VM_NAME" "${QA_CONFIG_FILE}" /tmp/qa.json

T1=$(kv_ssh "$VM_NAME" '
    echo --- os ---
    grep -E "VERSION_ID|PRETTY_NAME" /etc/os-release
    echo --- util-linux ---
    sfdisk --version
    wipefs --version
    echo --- sfdisk --relocate ---
    sfdisk --help 2>&1 | grep -o -- --relocate || echo MISSING
    echo --- headless --dry-run ---
    sudo /tmp/knuckle --headless --dry-run --config /tmp/qa.json --log-file /tmp/knuckle-dryrun.log 2>&1
    echo --- progress steps ---
    sudo cat /tmp/knuckle-dryrun.log 2>/dev/null | grep -o "\"msg\":\"[^\"]*\"" | head -12
' 2>&1) || T1="DRY_RUN_ERROR"

DRY_OK=$(echo "$T1" | grep -c "Installation complete" || true)
# Also check for JSON parse errors — indicates config was malformed
JSON_ERR=$(echo "$T1" | grep -c "parsing config JSON\|invalid character" || true)
[[ $JSON_ERR -gt 0 ]] && DRY_OK=0
{
  echo "### Tier 1 — Installer VM: tool check + dry-run (VM ${VM_NAME})"
  echo
  echo '```'
  echo "$T1"
  echo '```'
  echo
  [[ $DRY_OK -gt 0 ]] && echo "**✅ TIER 1 PASS**" || echo "**❌ TIER 1 FAIL**"
  echo
} >> "$REPORT"

if [[ $DRY_OK -eq 0 ]]; then
  echo "**Verdict: ❌ FAIL — dry-run did not complete**" >> "$REPORT"
  _fetch_artifacts
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Dry-run failed" || true
  cat "$REPORT"; exit 1
fi

# ── 9. Security bad-input tests (Tier 1+) ────────────────────────────────────
if [[ $DO_SECURITY -eq 1 ]]; then
  log "Security: bad-input rejection tests..."

  cat > /tmp/qa-sec-tests-${PR}.sh << 'SECSCRIPT'
#!/bin/bash
set -euo pipefail
KNUCKLE="sudo /tmp/knuckle"
FAIL=0

run_expect_fail() {
  local desc="$1"; local cfg="$2"
  printf '%s\n' "$cfg" > /tmp/qa-sec-test.json
  if $KNUCKLE --headless --dry-run --config /tmp/qa-sec-test.json --log-file /tmp/qa-sec.log 2>&1; then
    echo "FAIL (accepted): $desc"
    FAIL=1
  else
    echo "PASS (rejected): $desc"
  fi
}

run_expect_pass() {
  local desc="$1"; local cfg="$2"
  printf '%s\n' "$cfg" > /tmp/qa-sec-test.json
  if $KNUCKLE --headless --dry-run --config /tmp/qa-sec-test.json --log-file /tmp/qa-sec.log 2>&1; then
    echo "PASS (accepted): $desc"
  else
    echo "FAIL (rejected): $desc"
    FAIL=1
  fi
}

BASE='{"channel":"stable","hostname":"qa-sec","timezone":"UTC","network":{"mode":"dhcp"},"disk":"/dev/vdb","update_strategy":"off","reboot":false}'

run_expect_fail "plaintext password" \
  "$(echo "$BASE" | python3 -c "import sys,json; d=json.load(sys.stdin); d['users']=[{'username':'core','password':'hunter2'}]; print(json.dumps(d))")"

run_expect_fail "malformed SSH key" \
  "$(echo "$BASE" | python3 -c "import sys,json; d=json.load(sys.stdin); d['users']=[{'username':'core','ssh_keys':['not-a-key']}]; print(json.dumps(d))")"

run_expect_fail "empty username" \
  "$(echo "$BASE" | python3 -c "import sys,json; d=json.load(sys.stdin); d['users']=[{'username':'','ssh_keys':['ssh-ed25519 AAAA test@qa']}]; print(json.dumps(d))")"

run_expect_pass "valid crypt hash" \
  "$(echo "$BASE" | python3 -c "import sys,json; d=json.load(sys.stdin); d['users']=[{'username':'core','password':r'\$6\$rounds=4096\$testsalt\$hashhash123'}]; print(json.dumps(d))")"

run_expect_pass "valid SSH key" \
  "$(echo "$BASE" | python3 -c "import sys,json; d=json.load(sys.stdin); d['users']=[{'username':'core','ssh_keys':['ssh-ed25519 AAAA test@qa']}]; print(json.dumps(d))")"

echo "SECURITY_TESTS_DONE fail_count=${FAIL}"
exit $FAIL
SECSCRIPT

  _scp_to $GOPTS /tmp/qa-sec-tests-${PR}.sh "$GHOST:${WORK_REMOTE}/sec-tests.sh"
  kv_scp_to_vm "$VM_NAME" "/tmp/qa-sec-tests-${PR}.sh" /tmp/sec-tests.sh

  SEC=$(kv_ssh "$VM_NAME" 'bash /tmp/sec-tests.sh 2>&1' 2>&1) || SEC_OK=0
  SEC_OK=$(echo "$SEC" | grep -c "SECURITY_TESTS_DONE fail_count=0" || true)

  {
    echo "### Security — Bad Input Rejection"
    echo
    echo '```'
    echo "$SEC"
    echo '```'
    echo
    [[ $SEC_OK -gt 0 ]] && echo "**✅ SECURITY TESTS PASS**" || echo "**❌ SECURITY TESTS FAIL**"
    echo
  } >> "$REPORT"

  if [[ $SEC_OK -eq 0 ]]; then
    echo "**Verdict: ❌ FAIL — security regression**" >> "$REPORT"
    _fetch_artifacts
    _file_issue_on_fail "$REPORT" "$RUNDIR" "Security regression: bad input accepted" || true
    cat "$REPORT"; exit 1
  fi
fi

[[ $TIER -lt 3 ]] && {
  echo "**Verdict: ✅ PASS**" >> "$REPORT"
  _fetch_artifacts
  log "Done (tier 1)"
  cat "$REPORT"; exit 0
}

# ── 10. Tier 3 — real headless install ───────────────────────────────────────
log "Tier 3: real headless install..."

INSTALL_OUT=$(kv_ssh "$VM_NAME" '
  sudo /tmp/knuckle --headless --config /tmp/qa.json \
    --log-file /tmp/knuckle-install.log 2>&1
  echo INSTALL_EXIT_CODE=$?
' 2>&1) || true
INSTALL_DONE=$(echo "$INSTALL_OUT" | grep -c "INSTALL_EXIT_CODE=0" || true)

{
  echo "### Tier 3 — Headless install"
  echo
  echo '```'
  echo "$INSTALL_OUT"
  echo '```'
  echo
} >> "$REPORT"

# Retrieve install log from VM
kv_ssh "$VM_NAME" 'sudo cat /tmp/knuckle-install.log 2>/dev/null' \
  > "${RUNDIR}/knuckle-install.log" 2>/dev/null || true
{
  echo "<details><summary>knuckle-install.log</summary>"
  echo
  echo '```'
  cat "${RUNDIR}/knuckle-install.log" | tail -40
  echo '```'
  echo
  echo "</details>"
  echo
  [[ $INSTALL_DONE -gt 0 ]] && echo "**✅ INSTALL COMPLETE**" || echo "**❌ INSTALL FAILED**"
  echo
} >> "$REPORT"

if [[ $INSTALL_DONE -eq 0 ]]; then
  _fetch_artifacts
  echo "**Verdict: ❌ FAIL — install did not complete**" >> "$REPORT"
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Headless install failed" || true
  cat "$REPORT"; exit 1
fi

# ── 11. Boot installed Flatcar ────────────────────────────────────────────────
log "Booting installed Flatcar (delete installer VM, boot-only)..."
kv_boot_installed "$VM_NAME"
kv_wait_ready "$VM_NAME" 180 || {
  { echo "### Installed System Boot"; echo "**⛔ INSTALLED SYSTEM DID NOT BOOT**"; echo; echo "**Verdict: ❌ FAIL**"; } >> "$REPORT"
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Installed Flatcar did not boot" || true
  cat "$REPORT"; exit 1
}

kv_wait_ssh "$VM_NAME" 180 || {
  { echo "### Installed System Boot"; echo "**⛔ SSH TIMEOUT — installed Flatcar did not come up**"; echo; echo "**Verdict: ❌ FAIL**"; } >> "$REPORT"
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Installed Flatcar SSH never ready" || true
  cat "$REPORT"; exit 1
}
log "Installed system online"

# ── 12. Domain assertions (run inside the booted installed system) ─────────────
log "Running domain assertions..."

# Build the assertion script locally — clean, no escaping hell
ASSERT_SCRIPT="/tmp/knuckle-qa-assert-${PR}.sh"

cat > "$ASSERT_SCRIPT" << ASSERT_SCRIPT_EOF
#!/bin/bash
# Domain assertions for PR #${PR}: ${TITLE}
# Run inside the booted installed Flatcar (not the installer).
# Note: -e is intentionally omitted so all assertions run even when one fails.
# The FAIL counter tracks failures; set -e would exit on the first mismatch
# and truncate the report before recording later evidence.
set -uo pipefail
FAIL=0

check() {
  local desc="\$1"; shift
  echo "=== \${desc} ==="
  if "\$@" 2>&1; then
    true
  else
    echo "FAIL: \${desc}"
    FAIL=1
  fi
  echo ""
}

must_exist() {
  local path="\$1"
  if [ -e "\$path" ]; then
    ls -lah "\$path" 2>&1
  else
    echo "FAIL: \${path} NOT FOUND"
    FAIL=1
  fi
}

# ── Baseline: always ──────────────────────────────────────────────────────────
echo "=== BASELINE: OS identity (installed system, not installer) ==="
grep -E "VERSION_ID|PRETTY_NAME" /etc/os-release
echo ""

echo "=== BASELINE: hostname matches config ==="
ACTUAL_HOST=\$(hostname)
echo "\${ACTUAL_HOST}"
if [ "\${ACTUAL_HOST}" != "qa-pr-${PR}" ]; then
  echo "FAIL: hostname mismatch: got '\${ACTUAL_HOST}', want 'qa-pr-${PR}'"
  echo "  (Ignition hostname field may not have propagated — check knuckle-install.log)"
  FAIL=1
else
  echo "ok"
fi
echo ""

echo "=== BASELINE: core user SSH key (Ignition applied) ==="
must_exist /home/core/.ssh/authorized_keys
wc -l /home/core/.ssh/authorized_keys
echo ""

ASSERT_SCRIPT_EOF

# Append domain-specific assertions based on labels
_has "domain:install" && cat >> "$ASSERT_SCRIPT" << 'INSTALL_ASSERTS'

# ── domain:install ────────────────────────────────────────────────────────────
echo "=== ASSERT [install]: GPT partition table intact ==="
sudo sfdisk -l /dev/vda 2>&1 | head -15
# GPT must be present — sfdisk exits 0 only when label is valid
sudo sfdisk -l /dev/vda 2>&1 | grep -q "^Disk label type: gpt\|^Disklabel type: gpt" || {
  echo "FAIL: GPT label not found — partition table may be corrupt"
  FAIL=1
}
echo ""

echo "=== ASSERT [install]: /dev/disk/by-id populated ==="
ls -la /dev/disk/by-id/ 2>&1 | grep -v "^total" | head -5
ls /dev/disk/by-id/ | grep -v "^$" | wc -l | xargs echo "by-id entries:"
echo ""

INSTALL_ASSERTS

echo "$TITLE $LABELS" | grep -qi "swap" && cat >> "$ASSERT_SCRIPT" << 'SWAP_ASSERTS'

# ── swap feature ──────────────────────────────────────────────────────────────
echo "=== ASSERT [swap]: /var/swapfile exists at mode 0600 ==="
must_exist /var/swapfile
stat -c "%a %n" /var/swapfile | grep -q "^600 " || { echo "FAIL: swapfile not mode 0600"; FAIL=1; }
echo ""

echo "=== ASSERT [swap]: swapon shows active swap ==="
swapon --show 2>&1
swapon --show 2>/dev/null | grep -q "swapfile" || { echo "FAIL: swapfile not in swapon --show"; FAIL=1; }
echo ""

echo "=== ASSERT [swap]: knuckle-create-swapfile.service completed ==="
systemctl status knuckle-create-swapfile.service 2>&1 | grep -E "Active:|Loaded:"
systemctl is-active knuckle-create-swapfile.service | grep -q "active" || {
  echo "FAIL: knuckle-create-swapfile.service did not complete successfully"
  FAIL=1
}
echo ""

echo "=== ASSERT [swap]: free shows non-zero Swap ==="
free -h 2>&1
free 2>/dev/null | grep -q "^Swap:.*[1-9]" || { echo "FAIL: no swap in free output"; FAIL=1; }
echo ""

SWAP_ASSERTS

echo "$TITLE $LABELS" | grep -qi "tailscale" && cat >> "$ASSERT_SCRIPT" << 'TS_ASSERTS'

# ── tailscale feature ─────────────────────────────────────────────────────────
echo "=== ASSERT [tailscale]: /etc/tailscale/tailscale.env exists at mode 0600 ==="
must_exist /etc/tailscale/tailscale.env
MODE=$(stat -c "%a" /etc/tailscale/tailscale.env 2>/dev/null || echo "MISSING")
[ "$MODE" = "600" ] || { echo "FAIL: mode is ${MODE}, expected 600"; FAIL=1; }
echo "mode: ${MODE}"
echo ""

echo "=== ASSERT [tailscale]: tailscaled.service is enabled ==="
systemctl is-enabled tailscaled.service 2>&1
systemctl is-enabled tailscaled.service 2>/dev/null | grep -q "enabled" || {
  echo "FAIL: tailscaled.service not enabled"
  FAIL=1
}
echo ""

echo "=== ASSERT [tailscale]: knuckle-tailscale-up.service is enabled ==="
systemctl is-enabled knuckle-tailscale-up.service 2>&1
systemctl is-enabled knuckle-tailscale-up.service 2>/dev/null | grep -q "enabled" || {
  echo "FAIL: knuckle-tailscale-up.service not enabled"
  FAIL=1
}
echo ""

TS_ASSERTS

_has "domain:ignition" && cat >> "$ASSERT_SCRIPT" << 'IGN_ASSERTS'

# ── domain:ignition ───────────────────────────────────────────────────────────
echo "=== ASSERT [ignition]: update strategy applied ==="
grep -i "strategy" /etc/flatcar/update.conf 2>/dev/null || echo "(update.conf not present — expected for update_strategy:off)"
echo ""

echo "=== ASSERT [ignition]: timezone link correct ==="
ls -la /etc/localtime 2>&1
echo ""

IGN_ASSERTS

_has "domain:sysext" && cat >> "$ASSERT_SCRIPT" << 'SYSEXT_ASSERTS'

# ── domain:sysext ──────────────────────────────────────────────────────────────
echo "=== ASSERT [sysext]: /etc/extensions populated ==="
ls /etc/extensions/ 2>&1
ls /etc/extensions/ 2>/dev/null | grep -q "\.raw$" || {
  echo "FAIL: no .raw files in /etc/extensions/"
  FAIL=1
}
echo ""

echo "=== ASSERT [sysext]: systemd-sysext status ==="
sudo systemd-sysext status 2>&1 | head -10
echo ""

SYSEXT_ASSERTS

# Always: final verdict
cat >> "$ASSERT_SCRIPT" << 'FINAL'

echo "=== ASSERTION SUMMARY ==="
if [ $FAIL -eq 0 ]; then
  echo "ALL_ASSERTIONS_PASS"
else
  echo "ASSERTIONS_FAILED fail_count=${FAIL}"
fi
exit $FAIL
FINAL

# SCP assertion script to ghost for artifact storage, then directly into the VM
_scp_to $GOPTS "$ASSERT_SCRIPT" "$GHOST:${WORK_REMOTE}/assert.sh" || true
kv_scp_to_vm "$VM_NAME" "$ASSERT_SCRIPT" /tmp/assert.sh 2>/dev/null || true
ASSERT_OUT=$(kv_ssh "$VM_NAME" 'bash /tmp/assert.sh 2>&1' 2>&1) || true
ASSERT_OK=$(echo "$ASSERT_OUT" | grep -c "ALL_ASSERTIONS_PASS" || true)
ASSERT_FAILS=$(echo "$ASSERT_OUT" | grep -c "^FAIL:" || true)

{
  echo "### Tier 3 — Installed Flatcar: domain assertions"
  echo
  echo '```'
  echo "$ASSERT_OUT"
  echo '```'
  echo
  if [[ $ASSERT_OK -gt 0 ]]; then
    echo "**✅ ALL DOMAIN ASSERTIONS PASS**"
  else
    echo "**❌ DOMAIN ASSERTIONS FAILED** (${ASSERT_FAILS} failure(s) — see FAIL: lines above)"
  fi
  echo
} >> "$REPORT"

# Cleanup handled by trap (_cleanup_kv calls kv_delete)

# Fetch all artifacts from ghost
_fetch_artifacts

# ── 13. Final verdict ─────────────────────────────────────────────────────────
{
  echo "---"
  echo
  echo "**Artifacts:** \`.qa/runs/${RUN_ID}/\`"
  echo
  if [[ $ASSERT_OK -gt 0 ]]; then
    echo "**Verdict: ✅ PASS** — installed system verified, all domain assertions clean"
  else
    echo "**Verdict: ❌ FAIL** — ${ASSERT_FAILS} assertion(s) failed (see above)"
  fi
} >> "$REPORT"

if [[ $ASSERT_OK -eq 0 ]]; then
  _file_issue_on_fail "$REPORT" "$RUNDIR" "Domain assertions failed (${ASSERT_FAILS} failure(s))" || true
fi

log "Artifacts: ${RUNDIR}/"
cat "$REPORT"
[[ $ASSERT_OK -gt 0 ]] && exit 0 || exit 1
