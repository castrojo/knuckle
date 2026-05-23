#!/usr/bin/env bash
# KubeVirt VM helpers — sourced by qa-test-pr.sh
# All operations run on ghost via SSH. kubectl/virtctl only — no QEMU process management.
# Requires: GHOST, GOPTS, KUBEVIRT_NS (defaults to knuckle-test)
set -euo pipefail

KUBEVIRT_NS="${KUBEVIRT_NS:-knuckle-test}"
FLATCAR_BASE="${QA_FLATCAR_BASE:-/var/tmp/knuckle-test/flatcar_base.img}"

# ControlMaster: share a single connection to ghost across all SSH/SCP calls.
# Saves ~400 TCP connections per Tier 3 run and avoids sshd MaxStartups throttling.
_KV_CM_PATH="/tmp/.ssh-cm-knuckle-${$}"
export GOPTS="${GOPTS} -o ControlMaster=auto -o ControlPath=${_KV_CM_PATH} -o ControlPersist=120"

_kube() { ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} $*"; }
_vc()   { ssh $GOPTS "$GHOST" "virtctl -n ${KUBEVIRT_NS} $*"; }

# kv_prepare_disk <name>
# Prepare per-VM installer and target disks on ghost.
#
# The qcow2 → raw conversion is expensive (~30s, 13 GB). Cache it as
# flatcar-base.raw (shared across all test runs). Each VM gets a
# reflink copy (--reflink=auto: instant CoW on btrfs/XFS, falls back
# to file copy on other filesystems) so writes never touch the cache.
# Fixes #252: previously every run did a full re-convert.
kv_prepare_disk() {
  local name="$1"
  local base="/var/tmp/knuckle-test/flatcar-base.raw"  # shared cache
  local dst="/var/tmp/knuckle-test/${name}-raw.img"    # per-VM reflink copy
  local tgt="/var/tmp/knuckle-test/${name}-target.img" # per-VM install target
  ssh $GOPTS "$GHOST" "
    # One-time: convert qcow2 → raw (survives between runs)
    if [[ ! -f '${base}' ]]; then
      echo 'Converting Flatcar base to raw (one-time, cached)...'
      sudo qemu-img convert -p -f qcow2 -O raw '${FLATCAR_BASE}' '${base}'
      sudo chown qemu:qemu '${base}' && sudo chmod 664 '${base}'
      sudo chcon -t container_file_t '${base}'
    fi
    # Per-VM: reflink copy — instant on btrfs, no extra disk space used
    if [[ ! -f '${dst}' ]]; then
      sudo cp --reflink=auto '${base}' '${dst}'
      sudo chown qemu:qemu '${dst}' && sudo chmod 664 '${dst}'
      sudo chcon -t container_file_t '${dst}'
    fi
    if [[ ! -f '${tgt}' ]]; then
      sudo qemu-img create -f raw '${tgt}' 20G
      sudo chown qemu:qemu '${tgt}' && sudo chmod 664 '${tgt}'
      sudo chcon -t container_file_t '${tgt}'
    fi
  "
}

# kv_apply_vm <name>
# Apply VirtualMachine to cluster and start it.
# B3 FIX: runStrategy:Manual prevents controller auto-restart during disk mount.
kv_apply_vm() {
  local name="$1"
  local root_path="/var/tmp/knuckle-test/${name}-raw.img"
  local tgt_path="/var/tmp/knuckle-test/${name}-target.img"
  ssh $GOPTS "$GHOST" kubectl apply -f - << YAML
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: ${name}
  namespace: ${KUBEVIRT_NS}
spec:
  runStrategy: Manual
  template:
    metadata:
      labels:
        kubevirt.io/vm: ${name}
    spec:
      domain:
        cpu:
          cores: 2
        memory:
          guest: 2Gi
        devices:
          disks:
            - name: rootdisk
              bootOrder: 1
              disk:
                bus: virtio
            - name: targetdisk
              disk:
                bus: virtio
          interfaces:
            - name: default
              masquerade: {}
        machine:
          type: q35
      networks:
        - name: default
          pod: {}
      volumes:
        - name: rootdisk
          hostDisk:
            path: ${root_path}
            type: Disk
        - name: targetdisk
          hostDisk:
            path: ${tgt_path}
            type: Disk
YAML
  _vc "start ${name}"
}

# kv_inject_ssh_key <name>
# Stop VM, poll until VMI is gone, mount ROOT p9 via losetup, inject authorized_keys.
# Flatcar reads ignition via fw_cfg — cloudInitNoCloud silently ignored.
# Flatcar core UID=500. ROOT = partition 9 (p9) in Flatcar's GPT layout.
# Uses losetup -P to probe partition table dynamically — no hardcoded byte offset.
# TIMING: wait for VMI to exist before stopping (it may still be Scheduling),
# then wait up to 60s for VMI to disappear before mounting disk.
kv_inject_ssh_key() {
  local name="$1"
  local img="/var/tmp/knuckle-test/${name}-raw.img"
  local key; key=$(ssh $GOPTS "$GHOST" "cat ~/.ssh/id_ed25519.pub")

  # Phase 1: wait for VMI to appear (KubeVirt controller creates it asynchronously)
  local deadline=$(( $(date +%s) + 60 ))
  until _kube "get vmi ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && { echo "TIMEOUT: VMI ${name} never appeared before inject"; return 1; }
    sleep 2
  done

  # Phase 2: stop VM and wait for VMI to be gone (safe to mount disk only then)
  # Allow 5s for the stop request to be processed before polling.
  _vc "stop ${name}" 2>/dev/null || true
  sleep 5
  deadline=$(( $(date +%s) + 120 ))
  while _kube "get vmi ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && { echo "TIMEOUT: VMI ${name} did not stop for key injection"; return 1; }
    sleep 3
  done

  # Per-VM mount point avoids collision when two PRs test concurrently.
  # set -e + trap guarantee loopback cleanup even on mount failure (fixes: if mount fails
  # without set -e, bash continues and writes authorized_keys to the HOST filesystem).
  ssh $GOPTS "$GHOST" "
    set -euo pipefail
    MNT='/mnt/flatcar-${name}'
    LOOP=\$(sudo losetup -f --show -P '${img}')
    _cleanup() { sudo umount \"\${MNT}\" 2>/dev/null || true; sudo losetup -d \"\${LOOP}\" 2>/dev/null || true; }
    trap _cleanup EXIT
    sudo mkdir -p \"\${MNT}\"
    sudo mount \${LOOP}p9 \"\${MNT}\"
    sudo mkdir -p \"\${MNT}\"/home/core/.ssh
    printf '%s\n' '${key}' | sudo tee \"\${MNT}\"/home/core/.ssh/authorized_keys >/dev/null
    sudo chown -R 500:500 \"\${MNT}\"/home/core/.ssh
    sudo chmod 700 \"\${MNT}\"/home/core/.ssh
    sudo chmod 600 \"\${MNT}\"/home/core/.ssh/authorized_keys
  "
  _vc "start ${name}"
}

# kv_wait_ready <name> [timeout]
# B4 FIX: poll for VMI creation before kubectl wait (wait exits immediately if resource missing).
kv_wait_ready() {
  local name="$1"
  local timeout="${2:-120}"
  local deadline=$(( $(date +%s) + timeout ))
  until _kube "get vmi ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && {
      echo "TIMEOUT: VMI ${name} never created"
      _kube "describe vm ${name}" 2>/dev/null || true
      return 1
    }
    sleep 2
  done
  local remaining=$(( deadline - $(date +%s) ))
  [[ $remaining -le 5 ]] && remaining=5
  _kube "wait vmi ${name} --for=condition=Ready --timeout=${remaining}s"
}

# kv_ip <name>
# masquerade networking — .status.interfaces[0].ipAddress is the guest-internal
# NAT address (10.0.2.2), not routable from ghost. Use the virt-launcher pod IP instead.
# NOTE: uses $GOPTS explicitly so IdentitiesOnly forces the right SSH key.
kv_ip() {
  local name="$1"
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} get pod -l kubevirt.io/vm=${name} \
    -o jsonpath='{.items[0].status.podIP}'"
}

# kv_wait_ssh <name> [timeout]
# Poll until SSH inside the VM is actually accepting connections (not just VMI Ready).
# KubeVirt condition:Ready fires when the QEMU process starts, not when the guest
# OS has booted. Flatcar needs ~30-60s after VM start to boot and open sshd.
# Pod IP is re-resolved on each attempt — CNI may not have assigned it yet when
# polling starts (fixes stale-IP bug: issue #263).
kv_wait_ssh() {
  local name="$1"
  local timeout="${2:-120}"
  local deadline
  deadline=$(( $(date +%s) + timeout ))
  local ip
  echo "Waiting for SSH in VM ${name}..."
  until {
    ip=$(kv_ip "$name" 2>/dev/null)
    [[ -n "$ip" ]] && ssh $GOPTS "$GHOST" \
      "ssh -o ConnectTimeout=3 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
           -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 core@${ip} true"
  } &>/dev/null 2>&1; do
    if [[ $(date +%s) -ge $deadline ]]; then
      echo "TIMEOUT: SSH never ready in VM ${name} (last pod IP: ${ip:-unresolved})"
      return 1
    fi
    sleep 5
  done
  echo "SSH ready in VM ${name} at ${ip}"
}

# kv_ssh <name> <cmd>
# Run cmd on VM via ghost → pod-network SSH.
# Both the ghost hop and the inner VM SSH use GOPTS (IdentitiesOnly enforced).
kv_ssh() {
  local name="$1"; shift
  local ip; ip=$(kv_ip "$name")
  ssh $GOPTS "$GHOST" "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 core@${ip} '$*'"
}

# kv_scp_to_vm <name> <local_src> <remote_dst>
# SCP from dev machine into VM via ghost.
# GOPTS applied to both hops (dev→ghost, ghost→VM).
kv_scp_to_vm() {
  local name="$1" src="$2" dst="$3"
  local ip; ip=$(kv_ip "$name")
  local tmp="/tmp/_kv_upload_${$}_${name}"
  scp $GOPTS "$src" "${GHOST}:${tmp}"
  # Use && not ; so a failed scp still cleans up the temp file and exits non-zero.
  ssh $GOPTS "$GHOST" "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 ${tmp} core@${ip}:${dst} && rm -f ${tmp} || { rm -f ${tmp}; exit 1; }"
}

# kv_boot_installed <name>
# Delete the installer VM and create a fresh boot-only VM from the target disk.
# This mirrors the original QEMU approach: kill installer, boot installed disk alone.
# Patching the live VM spec in-place causes KubeVirt reconciler race conditions;
# a clean VM object with only the installed disk is more reliable.
kv_boot_installed() {
  local name="$1"
  local tgt_path="/var/tmp/knuckle-test/${name}-target.img"

  # Delete the installer VM object ONLY — NOT the disk files.
  # kv_delete removes both disks; we must preserve target.img (installed Flatcar).
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} delete vm ${name} \
    --ignore-not-found --wait=false" 2>/dev/null || true

  # Poll until BOTH VMI and VM objects are gone before disk reuse.
  # --wait=false returns when the delete is accepted, not when KubeVirt has released the disk.
  local deadline
  deadline=$(( $(date +%s) + 60 ))
  while _kube "get vmi ${name}" &>/dev/null 2>&1 || _kube "get vm ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && { echo "TIMEOUT: old VM/VMI ${name} did not stop before boot-installed"; return 1; }
    sleep 3
  done

  # Create a new minimal VM: only the installed target disk, no installer disk
  ssh $GOPTS "$GHOST" kubectl apply -f - << YAML
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: ${name}
  namespace: ${KUBEVIRT_NS}
spec:
  runStrategy: Manual
  template:
    metadata:
      labels:
        kubevirt.io/vm: ${name}
    spec:
      domain:
        cpu:
          cores: 2
        memory:
          guest: 2Gi
        devices:
          disks:
            - name: rootdisk
              bootOrder: 1
              disk:
                bus: virtio
          interfaces:
            - name: default
              masquerade: {}
        machine:
          type: q35
      networks:
        - name: default
          pod: {}
      volumes:
        - name: rootdisk
          hostDisk:
            path: ${tgt_path}
            type: Disk
YAML
  _vc "start ${name}"
}

# kv_delete <name>
# Delete VM and disk files. B5 FIX: --wait=false avoids 60s block during crash cleanup.
kv_delete() {
  local name="$1"
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} delete vm ${name} \
    --ignore-not-found --wait=false" 2>/dev/null || true
  # Detach any loopback device left over from kv_inject_ssh_key (leak on interrupted tests).
  # Also remove per-VM mount point if it was left mounted (belt + suspenders with the trap).
  ssh $GOPTS "$GHOST" "
    for loop in \$(sudo losetup -j '/var/tmp/knuckle-test/${name}-raw.img' 2>/dev/null | cut -d: -f1); do
      sudo umount /mnt/flatcar-${name} 2>/dev/null || true
      sudo losetup -d \"\$loop\" 2>/dev/null || true
    done
    sudo rm -rf /mnt/flatcar-${name} 2>/dev/null || true
    sudo rm -f /var/tmp/knuckle-test/${name}-raw.img  2>/dev/null || true
    sudo rm -f /var/tmp/knuckle-test/${name}-target.img 2>/dev/null || true
  " 2>/dev/null || true
}
