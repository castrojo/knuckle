#!/usr/bin/env bats
# Tests for scripts/fix-ghost-otel-process-noise.sh: config discovery, the
# embedded YAML patcher, error paths, and remote dispatch.
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/fix-ghost-otel-process-noise.bats

SCRIPT="$BATS_TEST_DIRNAME/../fix-ghost-otel-process-noise.sh"
ORIGINAL_PATH="$PATH"
WORK=""

setup() {
  WORK=$(mktemp -d "${TMPDIR:-/tmp}/fix-ghost-otel.XXXXXX")
  mkdir -p "$WORK/bin" "$WORK/home"

  export HOME="$WORK/home"
  export STUB_LOG="$WORK/calls.log"
  export STUB_EXECSTART=""
  export STUB_ACTIVE=0
  : >"$STUB_LOG"

  cat >"$WORK/bin/systemctl" <<'STUB'
#!/usr/bin/env bash
echo "systemctl $*" >>"$STUB_LOG"
case "$*" in
  *"-p ExecStart"*) printf '%s\n' "${STUB_EXECSTART:-}" ;;
  *"is-active --quiet"*) exit "${STUB_ACTIVE:-0}" ;;
  *"is-active"*) printf 'active\n' ;;
esac
exit 0
STUB

  cat >"$WORK/bin/journalctl" <<'STUB'
#!/usr/bin/env bash
echo "journalctl $*" >>"$STUB_LOG"
printf 'otelcol: permission denied reading /proc/1\n'
STUB

  cat >"$WORK/bin/sleep" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB

  cat >"$WORK/bin/ssh" <<'STUB'
#!/usr/bin/env bash
echo "ssh $*" >>"$STUB_LOG"
cat >"$(dirname "$STUB_LOG")/ssh-stdin"
STUB

  chmod +x "$WORK"/bin/*
  export PATH="$WORK/bin:$ORIGINAL_PATH"
}

teardown() {
  export PATH="$ORIGINAL_PATH"
  [ -n "$WORK" ] && /bin/rm -rf "$WORK"
}

# Writes a hostmetrics config containing a `process:` scraper and returns nothing.
write_config() {
  local path="$1"
  mkdir -p "$(dirname "$path")"
  cat >"$path" <<'YAML'
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:
      processes:
      process:
        mute_process_user_error: true
        metrics:
          process.cpu.time:
            enabled: true
exporters:
  otlp:
    endpoint: localhost:4317
YAML
}

run_apply() {
  run bash "$SCRIPT" --apply-local
}

# ── help ─────────────────────────────────────────────────────────────────────

@test "--help prints usage without touching systemd" {
  run bash "$SCRIPT" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"--apply-local"* ]]
  [[ "$output" == *"OTELCOL_USER_UNIT"* ]]
  [ ! -s "$STUB_LOG" ]
}

@test "-h is accepted as the help alias" {
  run bash "$SCRIPT" -h
  [ "$status" -eq 0 ]
  [[ "$output" == *"Usage:"* ]]
}

# ── config discovery ─────────────────────────────────────────────────────────

@test "config is taken from ExecStart --config=path (equals form)" {
  local cfg="$WORK/custom/agent.yaml"
  write_config "$cfg"
  export STUB_EXECSTART="/usr/bin/otelcol-agent --config=$cfg"

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $cfg"* ]]
}

@test "config is taken from ExecStart --config path (space form)" {
  local cfg="$WORK/custom/agent.yaml"
  write_config "$cfg"
  export STUB_EXECSTART="/usr/bin/otelcol-agent --config $cfg"

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $cfg"* ]]
}

@test "quoted ExecStart config path is unquoted before use" {
  local cfg="$WORK/custom/agent.yaml"
  write_config "$cfg"
  export STUB_EXECSTART="/usr/bin/otelcol-agent --config=\"$cfg\""

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $cfg"* ]]
}

@test "falls back to \$HOME/.config/otelcol-agent/config.yaml" {
  write_config "$HOME/.config/otelcol-agent/config.yaml"
  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $HOME/.config/otelcol-agent/config.yaml"* ]]
}

@test "falls back to \$HOME/.config/otelcol/config.yaml" {
  write_config "$HOME/.config/otelcol/config.yaml"
  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $HOME/.config/otelcol/config.yaml"* ]]
}

@test "falls back to \$HOME/.otelcol-agent.yaml" {
  write_config "$HOME/.otelcol-agent.yaml"
  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $HOME/.otelcol-agent.yaml"* ]]
}

@test "ExecStart path that does not exist falls through to candidates" {
  write_config "$HOME/.config/otelcol-agent/config.yaml"
  export STUB_EXECSTART="/usr/bin/otelcol-agent --config=/nonexistent/otel.yaml"

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Patched $HOME/.config/otelcol-agent/config.yaml"* ]]
}

@test "no discoverable config exits 1 with a diagnostic dump" {
  run_apply
  [ "$status" -eq 1 ]
  [[ "$output" == *"could not find config for systemd user unit otelcol-agent.service"* ]]
  grep -q -- "-p FragmentPath" "$STUB_LOG"
}

@test "OTELCOL_USER_UNIT overrides the queried unit" {
  export OTELCOL_USER_UNIT="ghost-otel.service"
  run_apply
  [ "$status" -eq 1 ]
  [[ "$output" == *"unit ghost-otel.service"* ]]
  grep -q "ghost-otel.service" "$STUB_LOG"
}

# ── YAML patching ────────────────────────────────────────────────────────────

@test "process scraper and its children are removed" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  write_config "$cfg"

  run_apply
  [ "$status" -eq 0 ]
  ! grep -qE '^\s+process:' "$cfg"
  ! grep -q "mute_process_user_error" "$cfg"
  ! grep -q "process.cpu.time" "$cfg"
}

@test "aggregate processes scraper and sibling scrapers survive" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  write_config "$cfg"

  run_apply
  [ "$status" -eq 0 ]
  grep -q "processes:" "$cfg"
  grep -q "cpu:" "$cfg"
  grep -q "memory:" "$cfg"
  grep -q "endpoint: localhost:4317" "$cfg"
}

@test "an explanatory comment replaces the removed scraper" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  write_config "$cfg"

  run_apply
  [ "$status" -eq 0 ]
  grep -q "process scraper disabled" "$cfg"
  [ "$(grep -c 'process scraper disabled' "$cfg")" -eq 1 ]
}

@test "a timestamped backup keeps the original content" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  write_config "$cfg"

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"Backup: $cfg.bak."* ]]

  local backup
  backup=$(printf '%s\n' "$HOME"/.config/otelcol-agent/config.yaml.bak.* | head -1)
  [ -f "$backup" ]
  grep -q "mute_process_user_error" "$backup"
}

@test "a config with no process scraper is left unchanged" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  mkdir -p "$(dirname "$cfg")"
  cat >"$cfg" <<'YAML'
receivers:
  hostmetrics:
    scrapers:
      cpu:
      memory:
YAML
  local before
  before=$(cat "$cfg")

  run_apply
  [ "$status" -ne 0 ]
  [[ "$output" == *"no hostmetrics scrapers.process entry found"* ]]
  [ "$(cat "$cfg")" = "$before" ]
}

@test "a process: key outside a scrapers block is not removed" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  mkdir -p "$(dirname "$cfg")"
  cat >"$cfg" <<'YAML'
processors:
  process:
    keep: true
receivers:
  hostmetrics:
    scrapers:
      process:
        mute_process_user_error: true
      processes:
YAML

  run_apply
  [ "$status" -eq 0 ]
  grep -q "keep: true" "$cfg"
  ! grep -q "mute_process_user_error" "$cfg"
  grep -q "processes:" "$cfg"
}

@test "scraper removal stops at the next top-level section" {
  local cfg="$HOME/.config/otelcol-agent/config.yaml"
  write_config "$cfg"

  run_apply
  [ "$status" -eq 0 ]
  grep -q "^exporters:" "$cfg"
  grep -q "^  otlp:" "$cfg"
}

# ── service restart ──────────────────────────────────────────────────────────

@test "the unit is restarted and its health rechecked after patching" {
  write_config "$HOME/.config/otelcol-agent/config.yaml"

  run_apply
  [ "$status" -eq 0 ]
  grep -q "systemctl --user restart otelcol-agent.service" "$STUB_LOG"
  grep -q "is-active --quiet otelcol-agent.service" "$STUB_LOG"
  [[ "$output" == *"Current otelcol-agent.service status: active"* ]]
}

@test "a unit that fails to come back active exits non-zero" {
  write_config "$HOME/.config/otelcol-agent/config.yaml"
  export STUB_ACTIVE=3

  run_apply
  [ "$status" -ne 0 ]
  [[ "$output" != *"Patched"* ]]
}

@test "permission-denied journal lines are reported after restart" {
  write_config "$HOME/.config/otelcol-agent/config.yaml"

  run_apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission denied"* ]]
  grep -q "journalctl --user -u otelcol-agent.service" "$STUB_LOG"
}

# ── remote dispatch ──────────────────────────────────────────────────────────

@test "a positional host is dispatched over ssh with the script on stdin" {
  run bash "$SCRIPT" qa@10.0.0.5
  [ "$status" -eq 0 ]
  grep -q "ssh .*qa@10.0.0.5" "$STUB_LOG"
  grep -q -- "--apply-local" "$STUB_LOG"
  grep -q "apply_local()" "$WORK/ssh-stdin"
}

@test "QA_HOST is used when no host argument is given" {
  export QA_HOST="ci@10.0.0.9"
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  grep -q "ci@10.0.0.9" "$STUB_LOG"
}

@test "the remote invocation forwards OTELCOL_USER_UNIT" {
  export QA_HOST="ci@10.0.0.9"
  export OTELCOL_USER_UNIT="ghost-otel.service"
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  grep -q "OTELCOL_USER_UNIT=ghost-otel.service" "$STUB_LOG"
}

@test "ssh runs with non-interactive host key options" {
  run bash "$SCRIPT" qa@10.0.0.5
  [ "$status" -eq 0 ]
  grep -q "StrictHostKeyChecking=no" "$STUB_LOG"
  grep -q "UserKnownHostsFile=/dev/null" "$STUB_LOG"
}
