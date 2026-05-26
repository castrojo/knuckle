#!/usr/bin/env bats
# Tests for scripts/iso-smoke.sh argument parsing and error paths.
# Requires: bats-core >= 1.5.0 (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/iso-smoke.bats

bats_require_minimum_version 1.5.0

SCRIPT="$BATS_TEST_DIRNAME/../iso-smoke.sh"

# ── Argument parsing ─────────────────────────────────────────────────────────

@test "no arguments prints usage and exits 1" {
  run bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"usage: iso-smoke.sh"* ]]
}

@test "only ISO path (missing OVMF) prints usage and exits 1" {
  run bash "$SCRIPT" /tmp/fake.iso
  [ "$status" -eq 1 ]
  [[ "$output" == *"usage: iso-smoke.sh"* ]]
}

# ── File existence checks ────────────────────────────────────────────────────

@test "non-existent ISO path exits 1 with error message" {
  run bash "$SCRIPT" /nonexistent/path.iso /tmp/fake-ovmf.fd
  [ "$status" -eq 1 ]
  [[ "$output" == *"ISO not found"* ]]
}

@test "non-existent OVMF path exits 1 with error message" {
  local iso
  iso=$(mktemp --suffix=.iso)
  run bash "$SCRIPT" "$iso" /nonexistent/ovmf.fd
  rm -f "$iso"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OVMF not found"* ]]
}

# ── Timeout validation ───────────────────────────────────────────────────────

@test "non-numeric timeout exits 1 with error" {
  local iso ovmf
  iso=$(mktemp --suffix=.iso)
  ovmf=$(mktemp --suffix=.fd)
  run bash "$SCRIPT" "$iso" "$ovmf" "abc"
  rm -f "$iso" "$ovmf"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Timeout must be an integer"* ]]
}

@test "negative timeout exits 1 with error" {
  local iso ovmf
  iso=$(mktemp --suffix=.iso)
  ovmf=$(mktemp --suffix=.fd)
  run bash "$SCRIPT" "$iso" "$ovmf" "-5"
  rm -f "$iso" "$ovmf"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Timeout must be an integer"* ]]
}

@test "fractional timeout exits 1 with error" {
  local iso ovmf
  iso=$(mktemp --suffix=.iso)
  ovmf=$(mktemp --suffix=.fd)
  run bash "$SCRIPT" "$iso" "$ovmf" "1.5"
  rm -f "$iso" "$ovmf"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Timeout must be an integer"* ]]
}

@test "valid numeric timeout is accepted (fails later at qemu)" {
  local iso ovmf
  iso=$(mktemp --suffix=.iso)
  ovmf=$(mktemp --suffix=.fd)
  # With valid timeout, script should proceed past validation
  # (will fail at qemu-img or qemu, not at timeout check)
  if command -v qemu-img >/dev/null 2>&1 && command -v qemu-system-x86_64 >/dev/null 2>&1; then
    run bash "$SCRIPT" "$iso" "$ovmf" "30"
  else
    run -127 bash "$SCRIPT" "$iso" "$ovmf" "30"
  fi
  rm -f "$iso" "$ovmf"
  # Should NOT fail with timeout validation error
  [[ "$output" != *"Timeout must be an integer"* ]]
}
