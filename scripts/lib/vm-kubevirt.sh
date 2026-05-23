#!/usr/bin/env bash
# KubeVirt VM helpers — sourced by qa-test-pr.sh
# All operations run on ghost via SSH. kubectl/virtctl only — no QEMU process management.
# Requires: GHOST, GOPTS, KUBEVIRT_NS (defaults to knuckle-test)
set -euo pipefail

KUBEVIRT_NS="${KUBEVIRT_NS:-knuckle-test}"
FLATCAR_BASE="${QA_FLATCAR_BASE:-/var/tmp/knuckle-test/flatcar_base.img}"

_kube() { ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} $*"; }
_vc()   { ssh $GOPTS "$GHOST" "virtctl -n ${KUBEVIRT_NS} $*"; }

# kv_prepare_disk <name>
# Expand flatcar_base.img to a named raw disk on ghost.
# B2 FIX: declare paths in outer function scope — never 'local' inside SSH heredoc.
kv_prepare_disk() {
  local name="$1"
  local dst="/var/tmp/knuckle-test/${name}-raw.img"
  local tgt="/var/tmp/knuckle-test/${name}-target.img"
  ssh $GOPTS "$GHOST" "
    if [[ ! -f '${dst}' ]]; then
      sudo qemu-img convert -p -f qcow2 -O raw '${FLATCAR_BASE}' '${dst}'
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
# Stop VM, poll until VMI is gone (B3 FIX), mount ROOT p9, inject authorized_keys.
# Flatcar reads ignition via fw_cfg — cloudInitNoCloud silently ignored.
# Flatcar core UID=500. ROOT = partition 9, offset 6513754112.
kv_inject_ssh_key() {
  local name="$1"
  local img="/var/tmp/knuckle-test/${name}-raw.img"
  local key; key=$(ssh $GOPTS "$GHOST" "cat ~/.ssh/id_ed25519.pub")
  _vc "stop ${name}" 2>/dev/null || true
  local deadline=$(( $(date +%s) + 30 ))
  while ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} get vmi ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && { echo "TIMEOUT: VMI stop"; return 1; }
    sleep 2
  done
  ssh $GOPTS "$GHOST" "
    sudo mkdir -p /mnt/flatcar-root
    sudo mount -o loop,offset=6513754112 '${img}' /mnt/flatcar-root
    sudo mkdir -p /mnt/flatcar-root/home/core/.ssh
    printf '%s\n' '${key}' | sudo tee /mnt/flatcar-root/home/core/.ssh/authorized_keys >/dev/null
    sudo chown -R 500:500 /mnt/flatcar-root/home/core/.ssh
    sudo chmod 700 /mnt/flatcar-root/home/core/.ssh
    sudo chmod 600 /mnt/flatcar-root/home/core/.ssh/authorized_keys
    sudo umount /mnt/flatcar-root
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
# B1 FIX: masquerade networking. Uses $GOPTS so IdentitiesOnly is enforced.
kv_ip() {
  local name="$1"
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} get pod -l kubevirt.io/vm=${name} \
    -o jsonpath='{.items[0].status.podIP}'"
}

# kv_ip <name>
# B1 FIX: masquerade networking — .status.interfaces[0].ipAddress is the guest-internal
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
# Also gives the CNI (flannel) time to set up routes to the pod IP.
kv_wait_ssh() {
  local name="$1"
  local timeout="${2:-120}"
  local deadline=$(( $(date +%s) + timeout ))
  local ip; ip=$(kv_ip "$name")
  echo "Waiting for SSH in VM ${name} at ${ip}..."
  until ssh $GOPTS "$GHOST" \
      "ssh -o ConnectTimeout=3 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
           -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 core@${ip} true" &>/dev/null 2>&1; do
    if [[ $(date +%s) -ge $deadline ]]; then
      echo "TIMEOUT: SSH never ready in VM ${name} (pod IP ${ip})"
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
  ssh $GOPTS "$GHOST" "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 ${tmp} core@${ip}:${dst}; rm -f ${tmp}"
}

# kv_swap_to_target <name>
# Stop installer VM, patch spec so rootdisk → target disk, restart.
# The installed Flatcar is now on target.img; booting it verifies the install.
kv_swap_to_target() {
  local name="$1"
  local tgt_path="/var/tmp/knuckle-test/${name}-target.img"
  _vc "stop ${name}" 2>/dev/null || true
  local deadline=$(( $(date +%s) + 30 ))
  while ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} get vmi ${name}" &>/dev/null 2>&1; do
    [[ $(date +%s) -ge $deadline ]] && { echo "TIMEOUT: VMI stop before swap"; return 1; }
    sleep 2
  done
  # Patch rootdisk path to the target (installed) disk
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} patch vm ${name} --type=json -p='
    [{\"op\":\"replace\",\"path\":\"/spec/template/spec/volumes/0/hostDisk/path\",\"value\":\"${tgt_path}\"}]'"
  _vc "start ${name}"
}

# kv_delete <name>
# Delete VM and disk files. B5 FIX: --wait=false avoids 60s block during crash cleanup.
kv_delete() {
  local name="$1"
  ssh $GOPTS "$GHOST" "kubectl -n ${KUBEVIRT_NS} delete vm ${name} \
    --ignore-not-found --wait=false" 2>/dev/null || true
  ssh $GOPTS "$GHOST" "
    sudo rm -f /var/tmp/knuckle-test/${name}-raw.img  2>/dev/null || true
    sudo rm -f /var/tmp/knuckle-test/${name}-target.img 2>/dev/null || true
  " 2>/dev/null || true
}
