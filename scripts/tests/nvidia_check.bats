#!/usr/bin/env bats
# Tests for scripts/nvidia_check.sh doc fetching and model comparison paths.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/nvidia_check.bats

SCRIPT="$BATS_TEST_DIRNAME/../nvidia_check.sh"
REPO_ROOT="$BATS_TEST_DIRNAME/../.."
ORIGINAL_PATH="$PATH"
MOCK_DIR=""

setup() {
  local workroot
  workroot="$BATS_TEST_DIRNAME/.bats-work"
  mkdir -p "$workroot"

  MOCK_DIR="$workroot/${BATS_TEST_NUMBER:-0}.$$.$RANDOM"
  mkdir -p "$MOCK_DIR"

  export PATH="$MOCK_DIR:$ORIGINAL_PATH"

  for tool in bash python3 grep sort awk sed; do
    ln -s "$(command -v "$tool")" "$MOCK_DIR/$tool"
  done
}

teardown() {
  export PATH="$ORIGINAL_PATH"
  /bin/rm -rf "$MOCK_DIR"
}

run_script() {
  run bash -c 'cd "$1" && bash "$2"' _ "$REPO_ROOT" "$SCRIPT"
}

mock_curl_json() {
  local markdown encoded
  markdown="$1"
  encoded=$(printf '%s' "$markdown" | base64 -w0)

  cat > "$MOCK_DIR/curl" <<EOF
#!/usr/bin/env bash
cat <<'JSON'
{"content":"$encoded"}
JSON
EOF
  chmod +x "$MOCK_DIR/curl"
}

mock_curl_raw() {
  cat > "$MOCK_DIR/curl" <<EOF
#!/usr/bin/env bash
cat <<'RAW'
$1
RAW
EOF
  chmod +x "$MOCK_DIR/curl"
}

@test "missing curl exits 2 with fetch error" {
  export PATH="$MOCK_DIR"

  run_script
  [ "$status" -eq 2 ]
  [[ "$output" == *"Fetching Flatcar NVIDIA docs..."* ]]
  [[ "$output" == *"ERROR: Could not fetch Flatcar NVIDIA docs from GitHub."* ]]
}

@test "docs with no nvidia driver series report none found and stay non-fatal" {
  mock_curl_json $'# Flatcar\n\nNo driver names are listed here.'

  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"Driver series mentioned in Flatcar NVIDIA docs:"* ]]
  [[ "$output" == *"(none found — docs may have changed structure)"* ]]
  [[ "$output" == *"✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs."* ]]
}

@test "single matching doc series exits 0, prints the detected series, and notes model-only entries" {
  mock_curl_json $'Use `nvidia-drivers-570-open` for modern GPUs.'

  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"570-open  (nvidia-drivers-570-open)"* ]]
  [[ "$output" == *"NOTE: 550-open is in model.go but not mentioned in current Flatcar docs"* ]]
  [[ "$output" == *"(This is normal — docs typically only show the recommended series)"* ]]
  [[ "$output" == *"✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs."* ]]
  [[ "$output" != *"ACTION REQUIRED"* ]]
}

@test "multiple doc series including an unknown one exits 1 and flags the missing model entry" {
  mock_curl_json $'Available series: `nvidia-drivers-570-open`, `nvidia-drivers-550-open`, and `nvidia-drivers-580-open`.'

  run_script
  [ "$status" -eq 1 ]
  [[ "$output" == *"570-open  (nvidia-drivers-570-open)"* ]]
  [[ "$output" == *"550-open  (nvidia-drivers-550-open)"* ]]
  [[ "$output" == *"580-open  (nvidia-drivers-580-open)"* ]]
  [[ "$output" == *"⚠ MISSING IN MODEL: 580-open"* ]]
  [[ "$output" == *"ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions"* ]]
}

@test "malformed docs payload exits 2 without crashing" {
  mock_curl_raw 'not-json'

  run_script
  [ "$status" -eq 2 ]
  [[ "$output" == *"ERROR: Could not fetch Flatcar NVIDIA docs from GitHub."* ]]
}
