#!/usr/bin/env bats
# Tests for scripts/lib/vm-kubevirt.sh behavior and error paths.
# Requires: bats-core >= 1.5.0 (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/vm-kubevirt.bats

bats_require_minimum_version 1.5.0

SCRIPT="$BATS_TEST_DIRNAME/../lib/vm-kubevirt.sh"
WORK_DIR=""

setup() {
  local workroot
  workroot="$BATS_TEST_DIRNAME/.bats-work"
  mkdir -p "$workroot"

  WORK_DIR="$workroot/vm-kubevirt-${BATS_TEST_NUMBER:-0}.$$.$RANDOM"
  mkdir -p "$WORK_DIR"

  export SCRIPT_PATH="$SCRIPT"
  export WORK_DIR
}

teardown() {
  /bin/rm -rf "$WORK_DIR"
}

@test "kv_apply_vm embeds ignition annotation when installer ignition exists" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

ssh() {
  local args="$*"
  printf '%s\n' "$args" >> "$WORK_DIR/ssh.log"
  if [[ "$args" == *"test -s '/var/tmp/knuckle-test/vm1-installer.ign'"* ]]; then
    return 0
  fi
  if [[ "$args" == *"cat '/var/tmp/knuckle-test/vm1-installer.ign'"* ]]; then
    echo '{"ignition":{"version":"3.4.0"}}'
    return 0
  fi
  if [[ "$args" == *"kubectl apply -f -"* ]]; then
    cat > "$WORK_DIR/apply.yaml"
    return 0
  fi
  if [[ "$args" == *"virtctl -n knuckle-test start vm1"* ]]; then
    return 0
  fi
  echo "unexpected ssh invocation: $args"
  return 1
}

kv_apply_vm "vm1"
BASH

  [ "$status" -eq 0 ]
  run grep -F "kubevirt.io/ignitiondata" "$WORK_DIR/apply.yaml"
  [ "$status" -eq 0 ]
  run grep -F '{"ignition":{"version":"3.4.0"}}' "$WORK_DIR/apply.yaml"
  [ "$status" -eq 0 ]
  run grep -F "path: /var/tmp/knuckle-test/vm1-target.img" "$WORK_DIR/apply.yaml"
  [ "$status" -eq 0 ]
  run grep -F "virtctl -n knuckle-test start vm1" "$WORK_DIR/ssh.log"
  [ "$status" -eq 0 ]
}

@test "kv_apply_vm omits ignition annotation when installer ignition is absent" {
  run bash <<'BASH'
set -euo pipefail
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

ssh() {
  local args="$*"
  if [[ "$args" == *"test -s '/var/tmp/knuckle-test/vm2-installer.ign'"* ]]; then
    return 1
  fi
  if [[ "$args" == *"cat '/var/tmp/knuckle-test/vm2-installer.ign'"* ]]; then
    echo "unexpected cat call" >&2
    return 1
  fi
  if [[ "$args" == *"kubectl apply -f -"* ]]; then
    cat > "$WORK_DIR/apply-no-ign.yaml"
    return 0
  fi
  if [[ "$args" == *"virtctl -n knuckle-test start vm2"* ]]; then
    return 0
  fi
  return 1
}

kv_apply_vm "vm2"
BASH

  [ "$status" -eq 0 ]
  run grep -F "kubevirt.io/ignitiondata" "$WORK_DIR/apply-no-ign.yaml"
  [ "$status" -ne 0 ]
}

@test "kv_inject_ssh_key short-circuits when installer ignition already exists" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

ssh() {
  local args="$*"
  printf '%s\n' "$args" >> "$WORK_DIR/inject.log"
  if [[ "$args" == *"test -s '/var/tmp/knuckle-test/vm3-installer.ign'"* ]]; then
    return 0
  fi
  echo "unexpected ssh invocation: $args"
  return 1
}

kv_inject_ssh_key "vm3"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"skipping offline SSH key injection"* ]]
  run wc -l < "$WORK_DIR/inject.log"
  [ "$status" -eq 0 ]
  [ "$output" -eq 1 ]
}

@test "kv_wait_ready times out when VMI is never created and describes VM" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

_kube() {
  printf '%s\n' "$1" >> "$WORK_DIR/wait-timeout.log"
  [[ "$1" == "describe vm vm-timeout" ]] && return 0
  return 1
}

kv_wait_ready "vm-timeout" 0
BASH

  [ "$status" -eq 1 ]
  [[ "$output" == *"TIMEOUT: VMI vm-timeout never created"* ]]
  run grep -F "describe vm vm-timeout" "$WORK_DIR/wait-timeout.log"
  [ "$status" -eq 0 ]
}

@test "kv_wait_ready enforces minimum 5 second wait timeout after VMI appears" {
  run bash <<'BASH'
set -euo pipefail
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

COUNT=0
NOW=100
date() {
  if [[ "${1:-}" == "+%s" ]]; then
    NOW=$((NOW + 1))
    echo "$NOW"
    return 0
  fi
  command date "$@"
}
sleep() { :; }

_kube() {
  printf '%s\n' "$1" >> "$WORK_DIR/wait-success.log"
  if [[ "$1" == "get vmi vm-ready" ]]; then
    COUNT=$((COUNT + 1))
    [[ "$COUNT" -ge 2 ]] && return 0
    return 1
  fi
  if [[ "$1" == "wait vmi vm-ready --for=condition=Ready --timeout=5s" ]]; then
    return 0
  fi
  return 1
}

kv_wait_ready "vm-ready" 1
BASH

  [ "$status" -eq 0 ]
  run grep -F "wait vmi vm-ready --for=condition=Ready --timeout=5s" "$WORK_DIR/wait-success.log"
  [ "$status" -eq 0 ]
}

@test "kv_wait_ssh timeout reports unresolved pod IP" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

kv_ip() { echo ""; }
kv_wait_ssh "vm-ssh-timeout" 0
BASH

  [ "$status" -eq 1 ]
  [[ "$output" == *"TIMEOUT: SSH never ready in VM vm-ssh-timeout (last pod IP: unresolved)"* ]]
}

@test "kv_wait_ssh retries until pod IP resolves and SSH succeeds" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

echo 100 > "$WORK_DIR/date-counter"
date() {
  if [[ "${1:-}" == "+%s" ]]; then
    local now
    now=$(<"$WORK_DIR/date-counter")
    now=$((now + 1))
    echo "$now" > "$WORK_DIR/date-counter"
    echo "$now"
    return 0
  fi
  command date "$@"
}
sleep() { :; }
kv_ip() {
  if [[ ! -f "$WORK_DIR/ip-ready" ]]; then
    : > "$WORK_DIR/ip-ready"
    echo ""
    return 0
  fi
  echo "10.0.2.15"
}
ssh() {
  local args="$*"
  printf '%s\n' "$args" >> "$WORK_DIR/ssh-ready.log"
  return 0
}

kv_wait_ssh "vm-ssh-ready" 3
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"SSH ready in VM vm-ssh-ready at 10.0.2.15"* ]]
  run grep -F "core@10.0.2.15 true" "$WORK_DIR/ssh-ready.log"
  [ "$status" -eq 0 ]
}

@test "kv_boot_installed creates boot-only VM from target disk and starts it" {
  run bash <<'BASH'
set -euo pipefail
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

_kube() { return 1; }
ssh() {
  local args="$*"
  printf '%s\n' "$args" >> "$WORK_DIR/boot.log"
  if [[ "$args" == *"delete vm vm4"* ]]; then
    return 0
  fi
  if [[ "$args" == *"kubectl apply -f -"* ]]; then
    cat > "$WORK_DIR/boot-apply.yaml"
    return 0
  fi
  if [[ "$args" == *"virtctl -n knuckle-test start vm4"* ]]; then
    return 0
  fi
  return 1
}

kv_boot_installed "vm4"
BASH

  [ "$status" -eq 0 ]
  run grep -F "path: /var/tmp/knuckle-test/vm4-target.img" "$WORK_DIR/boot-apply.yaml"
  [ "$status" -eq 0 ]
  run grep -F "name: targetdisk" "$WORK_DIR/boot-apply.yaml"
  [ "$status" -ne 0 ]
  run grep -F "virtctl -n knuckle-test start vm4" "$WORK_DIR/boot.log"
  [ "$status" -eq 0 ]
}

# ── kv_prepare_disk ───────────────────────────────────────────────────────────

@test "kv_prepare_disk creates raw and target disks on ghost" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
export FLATCAR_BASE="/var/tmp/flatcar_base.img"
source "$SCRIPT_PATH"

CALLS=""
ssh() {
  CALLS="$CALLS ssh:$*"
  # Simulate no cache files existing so all three branches run
  return 0
}

kv_prepare_disk "pr-42"
echo "calls:$CALLS"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"pr-42-raw.img"* ]]
  [[ "$output" == *"pr-42-target.img"* ]]
}

@test "kv_prepare_disk passes GOPTS to ssh" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
export FLATCAR_BASE="/var/tmp/flatcar_base.img"
source "$SCRIPT_PATH"

SSH_ARGS=""
ssh() { SSH_ARGS="$*"; return 0; }

kv_prepare_disk "test-vm"
echo "args:$SSH_ARGS"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"-o IdentitiesOnly=yes"* ]]
  [[ "$output" == *"ghost.local"* ]]
}

# ── kv_write_ignition ─────────────────────────────────────────────────────────

@test "kv_write_ignition uses provided key without fetching from ghost" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

LOG=$(mktemp)
# ssh in a pipeline runs in a subshell — write to a file so the parent can read it
ssh() {
  echo "$*" >> "$LOG"
  if [[ "$*" == *"cat >"* ]]; then cat > /dev/null; fi
  return 0
}
export -f ssh
export LOG

kv_write_ignition "vm1" "ssh-ed25519 AAAA test@host"
echo "log:$(cat "$LOG")"
rm -f "$LOG"
BASH

  [ "$status" -eq 0 ]
  # key-fetch path must NOT have been taken
  [[ "$output" != *"cat ~/.ssh/id_ed25519.pub"* ]]
  # write-ignition path must have been taken (installer ign path in ssh args)
  [[ "$output" == *"vm1-installer.ign"* ]]
}

@test "kv_write_ignition fetches key from ghost when none provided" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

ssh() {
  if [[ "$*" == *"cat ~/.ssh/id_ed25519.pub"* ]]; then
    echo "ssh-ed25519 AAAA ghost@host"
    return 0
  fi
  cat > /dev/null
  return 0
}

kv_write_ignition "vm2"
BASH

  [ "$status" -eq 0 ]
}

# ── kv_ip ─────────────────────────────────────────────────────────────────────

@test "kv_ip queries pod by kubevirt.io/vm label" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

SSH_ARGS=""
ssh() { SSH_ARGS="$*"; echo "10.244.1.5"; }

kv_ip "myvm"
echo "args:$SSH_ARGS"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"10.244.1.5"* ]]
  [[ "$output" == *"kubevirt.io/vm=myvm"* ]]
  [[ "$output" == *"podIP"* ]]
}

# ── kv_ssh ────────────────────────────────────────────────────────────────────

@test "kv_ssh routes command through ghost to core@<pod-ip>" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

SSH_CALLS=""
ssh() {
  SSH_CALLS="$SSH_CALLS|$*"
  # First call is kv_ip → return the pod IP
  if [[ "$*" == *"jsonpath"* ]]; then echo "10.244.2.7"; return 0; fi
  return 0
}

kv_ssh "myvm" "uname -a"
echo "calls:$SSH_CALLS"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"core@10.244.2.7"* ]]
  [[ "$output" == *"uname -a"* ]]
}

# ── kv_scp_to_vm ──────────────────────────────────────────────────────────────

@test "kv_scp_to_vm uploads via ghost then SCP to VM" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

CALLS=""
ssh() {
  CALLS="$CALLS|ssh:$*"
  if [[ "$*" == *"jsonpath"* ]]; then echo "10.244.3.1"; return 0; fi
  return 0
}
scp() { CALLS="$CALLS|scp:$*"; return 0; }

kv_scp_to_vm "myvm" "/local/file" "/remote/path"
echo "calls:$CALLS"
BASH

  [ "$status" -eq 0 ]
  # Should scp local→ghost first
  [[ "$output" == *"scp:"*"ghost.local"* ]]
  # Then ssh on ghost to scp ghost→VM
  [[ "$output" == *"core@10.244.3.1"* ]]
}

# ── kv_delete ─────────────────────────────────────────────────────────────────

@test "kv_delete issues kubectl delete and removes disk files" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

SSH_CMD=""
ssh() { SSH_CMD="$SSH_CMD|$*"; return 0; }

kv_delete "pr-99"
echo "cmd:$SSH_CMD"
BASH

  [ "$status" -eq 0 ]
  [[ "$output" == *"delete vm pr-99"* ]]
  [[ "$output" == *"pr-99-raw.img"* ]]
  [[ "$output" == *"pr-99-target.img"* ]]
  [[ "$output" == *"pr-99-installer.ign"* ]]
}

@test "kv_delete succeeds when VM is already gone" {
  run bash <<'BASH'
set -euo pipefail
exec 2>&1
export GHOST="ghost.local"
export GOPTS="-o IdentitiesOnly=yes"
export KUBEVIRT_NS="knuckle-test"
source "$SCRIPT_PATH"

# Simulate VM not found — kubectl delete returns non-zero for get, 0 for delete
ssh() {
  [[ "$*" == *"delete vm"* ]] && return 0
  [[ "$*" == *"rm -f"* ]] && return 0
  return 0
}

kv_delete "gone-vm"
BASH

  [ "$status" -eq 0 ]
}
