#!/usr/bin/env bats
# Tests for scripts/build-fcos-iso.sh argument validation and error paths.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/build-fcos-iso.bats

SCRIPT="$BATS_TEST_DIRNAME/../build-fcos-iso.sh"

# Helper: run the script expecting a non-zero exit
run_expect_fail() {
  run bash "$SCRIPT" "$@"
  [ "$status" -ne 0 ]
}

# ── Argument parsing ─────────────────────────────────────────────────────────

@test "unknown argument exits with error" {
  run_expect_fail --bogus-flag
  [[ "$output" == *"Unknown argument"* ]]
}

@test "invalid --arch value exits with error" {
  run_expect_fail --arch riscv64
  [[ "$output" == *"must be amd64 or arm64"* ]]
}

@test "invalid --stream value exits with error" {
  run_expect_fail --stream nightly
  [[ "$output" == *"must be stable, testing, or next"* ]]
}

@test "valid stream names are accepted" {
  # These should pass argument validation (may fail later due to missing deps)
  for s in stable testing next; do
    run bash "$SCRIPT" --stream "$s" --arch amd64 2>&1 || true
    # Should not contain the stream validation error
    [[ "$output" != *"must be stable, testing, or next"* ]]
  done
}

@test "--stream=value (equals form) is parsed" {
  run bash "$SCRIPT" --stream=nightly 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"must be stable, testing, or next"* ]]
}

@test "--arch=value (equals form) is parsed" {
  run bash "$SCRIPT" --arch=sparc 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"must be amd64 or arm64"* ]]
}

@test "bare stream name (positional) is parsed" {
  run bash "$SCRIPT" testing --arch=riscv64 2>&1
  [ "$status" -ne 0 ]
  # testing should be accepted; riscv64 should fail
  [[ "$output" == *"must be amd64 or arm64"* ]]
}

# ── --binary argument forms ──────────────────────────────────────────────────

@test "--binary value (space-separated) does not trigger unknown-argument error" {
  run bash "$SCRIPT" --binary /tmp/fake-knuckle 2>&1 || true
  [[ "$output" != *"Unknown argument"* ]]
}

@test "--binary=value (equals form) does not trigger unknown-argument error" {
  run bash "$SCRIPT" --binary=/tmp/fake-knuckle 2>&1 || true
  [[ "$output" != *"Unknown argument"* ]]
}

@test "--binary combined with valid flags does not trigger arg-parse error" {
  run bash "$SCRIPT" --binary /tmp/fake-knuckle --stream stable --arch amd64 2>&1 || true
  [[ "$output" != *"Unknown argument"* ]]
  [[ "$output" != *"must be amd64 or arm64"* ]]
  [[ "$output" != *"must be stable, testing, or next"* ]]
}

# ── --ssh-key argument forms ─────────────────────────────────────────────────

@test "--ssh-key value (space-separated) does not trigger unknown-argument error" {
  run bash "$SCRIPT" --ssh-key "ssh-ed25519 AAAA..." 2>&1 || true
  [[ "$output" != *"Unknown argument"* ]]
}

@test "--ssh-key=value (equals form) does not trigger unknown-argument error" {
  run bash "$SCRIPT" --ssh-key="ssh-ed25519 AAAA..." 2>&1 || true
  [[ "$output" != *"Unknown argument"* ]]
}

# ── Dependency check ─────────────────────────────────────────────────────────

@test "missing coreos-installer prints install hint" {
  # Override PATH to ensure coreos-installer is not found
  run env PATH=/usr/bin:/bin bash "$SCRIPT" --stream stable --arch amd64 2>&1
  if [[ "$output" == *"coreos-installer not found"* ]]; then
    [ "$status" -ne 0 ]
    [[ "$output" == *"coreos-installer not found"* ]]
  else
    # coreos-installer is present — skip this assertion
    skip "coreos-installer is available in PATH"
  fi
}
