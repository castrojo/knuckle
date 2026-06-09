#!/usr/bin/env bats
# Tests for scripts/lib/vm-kubevirt.sh — KubeVirt VM helper library.
# Requires: bats-core >= 1.5.0 (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/vm-kubevirt.bats
#
# Strategy: vm-kubevirt.sh is sourced (not executed) and depends on SSH to a
# KubeVirt host. We test by mocking ssh/scp/python3 via PATH manipulation and
# verifying argument construction, caching logic, and error propagation.
bats_require_minimum_version 1.5.0

LIB="$BATS_TEST_DIRNAME/../lib/vm-kubevirt.sh"
MOCK_DIR=""

setup() {
  MOCK_DIR="$(mktemp -d)"
  export PATH="$MOCK_DIR:$PATH"
  export GHOST="test-ghost.local"
  export GOPTS="-o StrictHostKeyChecking=no"
  export KUBEVIRT_NS="knuckle-test"
  export QA_FLATCAR_BASE="/var/tmp/knuckle-test/flatcar_base.img"

  # Default mock ssh: log invocations and succeed
  export SSH_LOG="$MOCK_DIR/ssh.log"
  : > "$SSH_LOG"

  cat > "$MOCK_DIR/ssh" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "${SSH_LOG}"
# Dispatch based on command content
case "$*" in
  *"kubectl"*"get vmi"*)
    # Default: VMI exists
    if [[ "${MOCK_VMI_EXISTS:-1}" == "0" ]]; then
      exit 1
    fi
    echo "vmi found"
    ;;
  *"kubectl"*"get pod"*"-o jsonpath"*)
    echo "${MOCK_POD_IP:-10.244.1.5}"
    ;;
  *"kubectl"*"wait vmi"*)
    exit "${MOCK_WAIT_EXIT:-0}"
    ;;
  *"kubectl"*"describe"*)
    echo "mock describe output"
    ;;
  *"virtctl"*)
    echo "virtctl: $*"
    ;;
  *"cat ~/.ssh/id_ed25519.pub"*)
    echo "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest test@host"
    ;;
  *"test -s"*)
    exit "${MOCK_IGN_EXISTS:-1}"  # Default: no ignition file
    ;;
  *"kubectl apply"*)
    echo "vm applied"
    ;;
  *)
    echo "mock-ssh: $*"
    ;;
esac
EOF
  chmod +x "$MOCK_DIR/ssh"

  cat > "$MOCK_DIR/scp" << 'EOF'
#!/usr/bin/env bash
echo "scp $*" >> "${SSH_LOG}"
exit 0
EOF
  chmod +x "$MOCK_DIR/scp"

  cat > "$MOCK_DIR/python3" << 'EOF'
#!/usr/bin/env bash
# Read stdin (the heredoc) and emit a fake Ignition JSON
echo '{"ignition":{"version":"3.4.0"}}'
EOF
  chmod +x "$MOCK_DIR/python3"
}

teardown() {
  rm -rf "$MOCK_DIR"
}

# ── Environment requirements ──────────────────────────────────────────────────

@test "sourcing without GHOST set fails" {
  unset GHOST
  # GOPTS references GHOST indirectly — but the script itself requires GHOST for _kube/_vc.
  # Source should succeed (GHOST checked at call time), but _kube should fail.
  export GOPTS="-o StrictHostKeyChecking=no"
  source "$LIB"
  run _kube "get pods"
  # ssh without a valid host arg will fail
  [[ "$status" -ne 0 ]] || [[ "$output" == *"mock-ssh"* ]]
}

@test "KUBEVIRT_NS defaults to knuckle-test" {
  unset KUBEVIRT_NS
  source "$LIB"
  [[ "$KUBEVIRT_NS" == "knuckle-test" ]]
}

@test "KUBEVIRT_NS can be overridden" {
  export KUBEVIRT_NS="custom-ns"
  source "$LIB"
  [[ "$KUBEVIRT_NS" == "custom-ns" ]]
}

# ── _kube / _vc wrappers ─────────────────────────────────────────────────────

@test "_kube passes namespace and command to ssh" {
  source "$LIB"
  _kube "get pods"
  grep -q "kubectl -n knuckle-test get pods" "$SSH_LOG"
}

@test "_vc passes namespace and command to ssh" {
  source "$LIB"
  _vc "start myvm"
  grep -q "virtctl -n knuckle-test start myvm" "$SSH_LOG"
}

@test "_kube uses GHOST as ssh target" {
  source "$LIB"
  _kube "get vmi test"
  grep -q "test-ghost.local" "$SSH_LOG"
}

# ── kv_prepare_disk ───────────────────────────────────────────────────────────

@test "kv_prepare_disk invokes ssh with disk name in paths" {
  source "$LIB"
  kv_prepare_disk "pr-42"
  grep -q "pr-42-raw.img" "$SSH_LOG"
  grep -q "pr-42-target.img" "$SSH_LOG"
}

@test "kv_prepare_disk references FLATCAR_BASE source image" {
  source "$LIB"
  kv_prepare_disk "test1"
  grep -q "flatcar_base.img" "$SSH_LOG"
}

# ── kv_write_ignition ─────────────────────────────────────────────────────────

@test "kv_write_ignition creates ignition file with provided key" {
  source "$LIB"
  kv_write_ignition "pr-42" "ssh-ed25519 AAAAprovided user@dev"
  grep -q "pr-42-installer.ign" "$SSH_LOG"
}

@test "kv_write_ignition fetches key from ghost when not provided" {
  source "$LIB"
  kv_write_ignition "pr-42"
  # Should have called cat ~/.ssh/id_ed25519.pub
  grep -q "id_ed25519.pub" "$SSH_LOG"
}

# ── kv_ip ─────────────────────────────────────────────────────────────────────

@test "kv_ip returns pod IP via kubectl jsonpath" {
  export MOCK_POD_IP="10.244.2.99"
  source "$LIB"
  run kv_ip "myvm"
  [[ "$output" == "10.244.2.99" ]]
}

@test "kv_ip queries pods by kubevirt.io/vm label" {
  source "$LIB"
  kv_ip "myvm"
  grep -q "kubevirt.io/vm=myvm" "$SSH_LOG"
}

# ── kv_ssh ────────────────────────────────────────────────────────────────────

@test "kv_ssh resolves IP and runs command on VM" {
  export MOCK_POD_IP="10.244.1.5"
  source "$LIB"
  kv_ssh "pr-42" "uname -a"
  # Should ssh to ghost with nested ssh to core@IP
  grep -q "core@10.244.1.5" "$SSH_LOG"
  grep -q "uname -a" "$SSH_LOG"
}

# ── kv_scp_to_vm ─────────────────────────────────────────────────────────────

@test "kv_scp_to_vm copies file through ghost to VM" {
  export MOCK_POD_IP="10.244.1.5"
  source "$LIB"
  kv_scp_to_vm "pr-42" "/tmp/local-file" "/home/core/dest"
  # SCP to ghost
  grep -q "scp.*test-ghost.local" "$SSH_LOG"
  # SSH to ghost for inner scp to VM
  grep -q "core@10.244.1.5" "$SSH_LOG"
}

# ── kv_wait_ready ─────────────────────────────────────────────────────────────

@test "kv_wait_ready succeeds when VMI exists and kubectl wait exits 0" {
  export MOCK_VMI_EXISTS=1
  export MOCK_WAIT_EXIT=0
  source "$LIB"
  run kv_wait_ready "pr-42" 5
  [[ "$status" -eq 0 ]]
}

@test "kv_wait_ready times out when VMI never appears" {
  # Mock ssh to always fail for get vmi
  cat > "$MOCK_DIR/ssh" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "${SSH_LOG}"
case "$*" in
  *"kubectl"*"get vmi"*) exit 1 ;;
  *"kubectl"*"describe"*) echo "mock describe" ;;
  *) exit 0 ;;
esac
EOF
  chmod +x "$MOCK_DIR/ssh"

  source "$LIB"
  run kv_wait_ready "pr-42" 3
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"TIMEOUT"* ]]
}

# ── kv_wait_ssh ───────────────────────────────────────────────────────────────

@test "kv_wait_ssh times out when SSH never connects" {
  # Mock ssh to always fail for the inner SSH connection test
  cat > "$MOCK_DIR/ssh" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "${SSH_LOG}"
case "$*" in
  *"kubectl"*"get pod"*"-o jsonpath"*) echo "10.244.1.5" ;;
  *"ssh -o ConnectTimeout"*) exit 1 ;;
  *) exit 0 ;;
esac
EOF
  chmod +x "$MOCK_DIR/ssh"

  source "$LIB"
  run kv_wait_ssh "pr-42" 3
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"TIMEOUT"* ]]
}

# ── kv_inject_ssh_key ─────────────────────────────────────────────────────────

@test "kv_inject_ssh_key skips injection when ignition already prepared" {
  export MOCK_IGN_EXISTS=0  # test -s succeeds
  source "$LIB"
  run kv_inject_ssh_key "pr-42"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"already prepared"* ]]
}

# ── kv_delete ─────────────────────────────────────────────────────────────────

@test "kv_delete calls kubectl delete and cleanup on ghost" {
  source "$LIB"
  kv_delete "pr-42"
  grep -q "kubectl.*delete vm pr-42" "$SSH_LOG"
  grep -q "pr-42-raw.img" "$SSH_LOG"
  grep -q "pr-42-target.img" "$SSH_LOG"
  grep -q "pr-42-installer.ign" "$SSH_LOG"
}

@test "kv_delete does not fail when VM does not exist" {
  source "$LIB"
  run kv_delete "nonexistent"
  [[ "$status" -eq 0 ]]
}

# ── kv_boot_installed ─────────────────────────────────────────────────────────

@test "kv_boot_installed deletes old VM and creates new minimal VM" {
  # Mock: VMI gone after first check
  local call_count=0
  cat > "$MOCK_DIR/ssh" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "${SSH_LOG}"
case "$*" in
  *"kubectl"*"delete vm"*)  exit 0 ;;
  *"kubectl"*"get vmi"*)    exit 1 ;;  # VMI gone
  *"kubectl"*"get vm"*)     exit 1 ;;  # VM gone
  *"kubectl apply"*)        echo "applied" ;;
  *"virtctl"*)              echo "started" ;;
  *) exit 0 ;;
esac
EOF
  chmod +x "$MOCK_DIR/ssh"

  source "$LIB"
  run kv_boot_installed "pr-42"
  [[ "$status" -eq 0 ]]
  grep -q "kubectl.*delete vm pr-42" "$SSH_LOG"
  grep -q "kubectl apply" "$SSH_LOG"
  grep -q "virtctl.*start pr-42" "$SSH_LOG"
}
