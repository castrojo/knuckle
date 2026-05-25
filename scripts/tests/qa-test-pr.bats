#!/usr/bin/env bats
# Tests for scripts/qa-test-pr.sh pure helper logic.

SCRIPT="$BATS_TEST_DIRNAME/../qa-test-pr.sh"

setup() {
  MOCK_BIN="$BATS_TEST_DIRNAME/.mock-bin-$$"
  mkdir -p "$MOCK_BIN"

  cat > "$MOCK_BIN/gh" <<'GH'
#!/usr/bin/env bash
printf '%s\n' "${MOCK_GH_OUTPUT:-{\"labels\":[]}}"
GH

  cat > "$MOCK_BIN/kubectl" <<'KUBECTL'
#!/usr/bin/env bash
printf '%s\n' "${MOCK_KUBECTL_OUTPUT:-kubectl stub}"
KUBECTL

  chmod +x "$MOCK_BIN/gh" "$MOCK_BIN/kubectl"
}

teardown() {
  rm -rf "$MOCK_BIN"
}

run_helper() {
  run env PATH="$MOCK_BIN:$PATH" SCRIPT="$SCRIPT" bash -c '
    source "$SCRIPT"
    "$@"
  ' bash "$@"
}

@test "should_skip_qa returns success when ci/skip label is present" {
  run_helper should_skip_qa "ci/skip, domain:ci"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "should_skip_qa returns non-zero when skip label is absent" {
  run_helper should_skip_qa "domain:ci, kind:test"
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}

@test "get_tier routes probe labels to tier 1" {
  run_helper get_tier "domain:probe"
  [ "$status" -eq 0 ]
  [ "$output" = "1" ]
}

@test "get_tier routes tui labels to tier 1" {
  run_helper get_tier "domain:tui"
  [ "$status" -eq 0 ]
  [ "$output" = "1" ]
}

@test "get_tier routes install labels to tier 3" {
  run_helper get_tier "domain:install"
  [ "$status" -eq 0 ]
  [ "$output" = "3" ]
}

@test "get_tier routes iso labels to tier 3" {
  run_helper get_tier "domain:iso"
  [ "$status" -eq 0 ]
  [ "$output" = "3" ]
}

@test "get_tier keeps ci-only labels at tier 0" {
  run_helper get_tier "domain:ci, kind:test"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "get_tier defaults to tier 0 when no domain labels are present" {
  run_helper get_tier "kind:test, docs"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "is_complex_pr flags XL PRs" {
  run_helper is_complex_pr "size:XL" 1 0
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "is_complex_pr flags PRs with more than four domains" {
  run_helper is_complex_pr "" 5 0
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "is_complex_pr flags workflow changes" {
  run_helper is_complex_pr "" 1 1
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "is_complex_pr allows small PRs without workflow changes" {
  run_helper is_complex_pr "size:M" 2 0
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}

@test "parse_pr_labels extracts label names from gh output" {
  json='{"labels":[{"name":"domain:probe"},{"name":"size:M"},{"name":"kind:test"}]}'
  run_helper parse_pr_labels "$json"
  [ "$status" -eq 0 ]
  [ "$output" = "domain:probe, size:M, kind:test" ]
}
