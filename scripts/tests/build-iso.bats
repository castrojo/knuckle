#!/usr/bin/env bats
# Tests for scripts/build-iso.sh argument validation and error paths.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/build-iso.bats

SCRIPT="$BATS_TEST_DIRNAME/../build-iso.sh"

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

@test "invalid --channel value exits with error" {
  run_expect_fail --channel nightly
  [[ "$output" == *"must be stable, beta, alpha, lts, or edge"* ]]
}

@test "arm64 + lts combination exits with error" {
  run_expect_fail --arch arm64 --channel lts
  [[ "$output" == *"LTS channel is not available for arm64"* ]]
}

@test "valid channel names are accepted" {
  # These should pass argument validation (may fail later due to missing deps)
  for ch in stable beta alpha lts edge; do
    run bash "$SCRIPT" --channel "$ch" --arch amd64 2>&1 || true
    # Should not contain the channel validation error
    [[ "$output" != *"must be stable, beta, alpha, lts, or edge"* ]]
  done
}

@test "--channel=value (equals form) is parsed" {
  run bash "$SCRIPT" --channel=nightly 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"must be stable, beta, alpha, lts, or edge"* ]]
}

@test "--arch=value (equals form) is parsed" {
  run bash "$SCRIPT" --arch=sparc 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"must be amd64 or arm64"* ]]
}

@test "bare channel name (positional) is parsed" {
  run bash "$SCRIPT" beta --arch=riscv64 2>&1
  [ "$status" -ne 0 ]
  # beta should be accepted; riscv64 should fail
  [[ "$output" == *"must be amd64 or arm64"* ]]
}
