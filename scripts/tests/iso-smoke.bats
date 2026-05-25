#!/usr/bin/env bats
# Tests for scripts/iso-smoke.sh argument validation and error paths.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/iso-smoke.bats

SCRIPT="$BATS_TEST_DIRNAME/../iso-smoke.sh"
TEST_ROOT="$BATS_TEST_DIRNAME/.test-artifacts/iso-smoke"

setup() {
  TEST_ID=$(printf '%s' "$BATS_TEST_NAME" | tr -cs '[:alnum:]' '-')
  WORKDIR="$TEST_ROOT/${BATS_TEST_NUMBER}-${TEST_ID}-$$"
  BIN_DIR="$WORKDIR/bin"
  TEST_LOG="$WORKDIR/stubs.log"
  ISO_PATH="$WORKDIR/test.iso"
  OVMF_PATH="$WORKDIR/OVMF.fd"

  mkdir -p "$BIN_DIR"
  export PATH="$BIN_DIR:$PATH"
  export TEST_LOG

  cd "$WORKDIR"
}

teardown() {
  cd "$BATS_TEST_DIRNAME"
  rm -rf "$WORKDIR"
}

write_stub() {
  local name=$1
  local body=$2

  printf '#!/usr/bin/env bash\nset -euo pipefail\n%s\n' "$body" >"$BIN_DIR/$name"
  chmod +x "$BIN_DIR/$name"
}

create_inputs() {
  mkdir -p "$WORKDIR"
  : >"$ISO_PATH"
  : >"$OVMF_PATH"
}

run_expect_fail() {
  run bash "$SCRIPT" "$@"
  [ "$status" -ne 0 ]
}

@test "usage error when no args provided exits 1" {
  run bash "$SCRIPT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"usage: iso-smoke.sh <iso-path> <ovmf-path> [timeout-seconds]"* ]]
}

@test "usage error when too few args provided exits 1" {
  run bash "$SCRIPT" "$ISO_PATH"

  [ "$status" -eq 1 ]
  [[ "$output" == *"usage: iso-smoke.sh <iso-path> <ovmf-path> [timeout-seconds]"* ]]
}

@test "missing ISO path exits 1" {
  : >"$OVMF_PATH"

  run_expect_fail "$ISO_PATH" "$OVMF_PATH"

  [ "$status" -eq 1 ]
  [[ "$output" == *"ISO not found: $ISO_PATH"* ]]
}

@test "missing OVMF path exits 1" {
  : >"$ISO_PATH"

  run_expect_fail "$ISO_PATH" "$OVMF_PATH"

  [ "$status" -eq 1 ]
  [[ "$output" == *"OVMF not found: $OVMF_PATH"* ]]
}

@test "non-integer timeout exits 1" {
  create_inputs

  run_expect_fail "$ISO_PATH" "$OVMF_PATH" not-a-number

  [ "$status" -eq 1 ]
  [[ "$output" == *"Timeout must be an integer number of seconds: not-a-number"* ]]
}

@test "qemu-img failure propagates its exit code" {
  create_inputs
  write_stub qemu-img $'echo "qemu-img failed" >&2\nexit 42'

  run bash "$SCRIPT" "$ISO_PATH" "$OVMF_PATH"

  [ "$status" -eq 42 ]
  [[ "$output" == *"qemu-img failed"* ]]
}

@test "serial log errors fail validation" {
  create_inputs
  write_stub qemu-img 'exit 0'
  write_stub qemu-system-x86_64 $'cat <<\'EOF\'\nsystemd-boot\nReached target initrd-root-device.target\nReached target initrd-usr-fs.target\nxd2root: simulated failure\nEOF'

  run_expect_fail "$ISO_PATH" "$OVMF_PATH" 5

  [ "$status" -eq 1 ]
  [[ "$output" == *"detected 1 xd2root/x2dauto error(s) in serial log"* ]]
}

@test "missing boot milestones fail validation" {
  create_inputs
  write_stub qemu-img 'exit 0'
  write_stub qemu-system-x86_64 $'cat <<\'EOF\'\nsystemd-boot\nEOF'

  run_expect_fail "$ISO_PATH" "$OVMF_PATH" 5

  [ "$status" -eq 1 ]
  [[ "$output" == *"iso-smoke FAILED: missing initrd-root-device.target,initrd-usr-fs.target"* ]]
}

@test "successful validation passes with mocked external commands" {
  create_inputs
  write_stub qemu-img 'exit 0'
  write_stub qemu-system-x86_64 $'cat <<\'EOF\'\nsystemd-boot\nReached target initrd-root-device.target\nReached target initrd-usr-fs.target\nEOF'

  run bash "$SCRIPT" "$ISO_PATH" "$OVMF_PATH" 5

  [ "$status" -eq 0 ]
  [[ "$output" == *"systemd-boot menu detected"* ]]
  [[ "$output" == *"initrd-root-device.target reached"* ]]
  [[ "$output" == *"initrd-usr-fs.target reached"* ]]
  [[ "$output" == *"✅ iso-smoke PASSED"* ]]
}
