#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
usage: iso-smoke.sh <iso-path> <ovmf-path> [timeout-seconds]

Boot a Knuckle installer ISO headlessly with QEMU and verify that:
- the systemd-boot menu appears on serial
- initrd-root-device.target is reached
- initrd-root-device.target is reached
- systemd-journald starts (proxy for initrd-usr-fs.target; journald lives in /usr so
  its successful start proves /usr was mounted — direct target logging stops once
  journald takes over the console on Flatcar 4230+)
- no xd2root/x2dauto errors appear in the serial log
EOF
}

ISO_PATH=${1:-}
OVMF_PATH=${2:-}
TIMEOUT_SECONDS=${3:-120}
QEMU_BIN=${QEMU_BIN:-qemu-system-x86_64}
TARGET_DISK=.vm/iso-smoke-target.qcow2
SERIAL_LOG=.vm/iso-smoke-serial.log
QEMU_PID=

if [[ -z "$ISO_PATH" || -z "$OVMF_PATH" ]]; then
  usage >&2
  exit 1
fi

if [[ ! -f "$ISO_PATH" ]]; then
  echo "ISO not found: $ISO_PATH" >&2
  exit 1
fi

if [[ ! -f "$OVMF_PATH" ]]; then
  echo "OVMF not found: $OVMF_PATH" >&2
  exit 1
fi

if [[ ! "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "Timeout must be an integer number of seconds: $TIMEOUT_SECONDS" >&2
  exit 1
fi

cleanup() {
  if [[ -n "$QEMU_PID" ]] && kill -0 "$QEMU_PID" 2>/dev/null; then
    kill "$QEMU_PID" 2>/dev/null || true
    wait "$QEMU_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

log_has() {
  local pattern=$1
  grep -aEq -- "$pattern" "$SERIAL_LOG"
}

mkdir -p .vm
rm -f "$TARGET_DISK" "$SERIAL_LOG"
qemu-img create -f qcow2 "$TARGET_DISK" 20G >/dev/null

printf '=== iso-smoke: headless ISO boot ===\n'
printf 'ISO: %s\n' "$ISO_PATH"
printf 'OVMF: %s\n' "$OVMF_PATH"
printf 'Log: %s\n\n' "$SERIAL_LOG"

"$QEMU_BIN" \
  -m 2048 \
  -cpu host \
  -enable-kvm \
  -drive if=pflash,format=raw,readonly=on,file="$OVMF_PATH" \
  -cdrom "$ISO_PATH" \
  -drive if=virtio,file="$TARGET_DISK",format=qcow2 \
  -nographic \
  >"$SERIAL_LOG" 2>&1 &
QEMU_PID=$!

menu_seen=0
root_seen=0
usr_seen=0

deadline=$((SECONDS + TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  if [[ -f "$SERIAL_LOG" ]]; then
    if (( menu_seen == 0 )) && log_has 'Knuckle.*Install Flatcar|Automatic boot in|systemd-boot'; then
      echo '  ✓ systemd-boot menu detected'
      menu_seen=1
    fi

    if (( root_seen == 0 )) && log_has 'Reached target (initrd-root-device\.target|Initrd Root Device\.)'; then
      echo '  ✓ initrd-root-device.target reached'
      root_seen=1
    fi

    if (( usr_seen == 0 )) && log_has 'Started systemd-journald\.service'; then
      echo '  ✓ systemd-journald started (usr-fs proxy: /usr mounted)'
      usr_seen=1
    fi

    error_count=$(grep -aiEc -- 'xd2root|x2dauto' "$SERIAL_LOG" || true)
    if (( error_count > 0 )); then
      echo "❌ detected $error_count xd2root/x2dauto error(s) in serial log"
      tail -40 "$SERIAL_LOG" || true
      exit 1
    fi

    if (( menu_seen == 1 && root_seen == 1 && usr_seen == 1 )); then
      echo
      echo '✅ iso-smoke PASSED'
      exit 0
    fi
  fi

  if ! kill -0 "$QEMU_PID" 2>/dev/null; then
    wait "$QEMU_PID" 2>/dev/null || true
    break
  fi

  sleep 1
done

missing_checks=()
(( menu_seen == 1 )) || missing_checks+=("systemd-boot menu")
(( root_seen == 1 )) || missing_checks+=("initrd-root-device.target")
(( usr_seen == 1 )) || missing_checks+=("systemd-journald start (usr-fs proxy)")

if (( ${#missing_checks[@]} == 0 )); then
  echo
  echo '✅ iso-smoke PASSED'
  exit 0
fi

echo
printf '❌ iso-smoke FAILED: missing %s\n' "$(IFS=', '; echo "${missing_checks[*]}")"
echo '--- serial log tail ---'
tail -60 "$SERIAL_LOG" || true
exit 1
