#!/usr/bin/env bash
# Disable the noisy OTel hostmetrics process scraper on the QA host.
#
# The ghost OTel collector runs as a systemd user service. The process scraper
# attempts to inspect every PID and emits large permission-denied batches for
# root-owned processes even with mute_process_user_error enabled. Keep aggregate
# process counts (the `processes` scraper) but remove per-process scraping.

set -euo pipefail

apply_local() {
  local unit=${OTELCOL_USER_UNIT:-otelcol-agent.service}
  local config backup

  find_config() {
    local exec_start cfg
    exec_start=$(systemctl --user show "$unit" -p ExecStart --value 2>/dev/null || true)

    cfg=$(printf '%s\n' "$exec_start" | grep -oE -- '--config(=| )[[:graph:]]+' | head -1 | sed -E 's/^--config[= ]//' || true)
    cfg=${cfg//\"/}
    cfg=${cfg//\'/}
    if [[ -n "${cfg:-}" && -f "$cfg" ]]; then
      printf '%s\n' "$cfg"
      return 0
    fi

    for candidate in \
      "$HOME/.config/otelcol-agent/config.yaml" \
      "$HOME/.config/otelcol-agent/config.yml" \
      "$HOME/.config/otelcol/config.yaml" \
      "$HOME/.otelcol-agent.yaml"; do
      if [[ -f "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done

    return 1
  }

  config=$(find_config) || {
    echo "ERROR: could not find config for systemd user unit $unit" >&2
    systemctl --user show "$unit" -p ExecStart -p FragmentPath -p DropInPaths --no-pager >&2 || true
    exit 1
  }

  backup="${config}.bak.$(date -u +%Y%m%dT%H%M%SZ)"
  cp -- "$config" "$backup"

  python3 - "$config" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
lines = path.read_text().splitlines(keepends=True)
out = []
in_scrapers = False
scrapers_indent = -1
skip_process = False
process_indent = -1
found = False
changed = False
inserted_comment = False

key_re = re.compile(r"^(?P<indent>\s*)(?P<key>[A-Za-z0-9_-]+):(?:\s*(?:#.*)?)?$")
process_re = re.compile(r"^(?P<indent>\s*)process:\s*(?:.*)?$")

for line in lines:
    stripped = line.strip()
    indent = len(line) - len(line.lstrip(" "))

    if skip_process:
        # Skip child settings belonging to the removed `process:` scraper.
        if stripped and not stripped.startswith("#") and indent <= process_indent:
            skip_process = False
        else:
            changed = True
            continue

    if stripped and not stripped.startswith("#"):
        match = key_re.match(line.rstrip("\n"))
        if in_scrapers and indent <= scrapers_indent and not (match and match.group("key") == "scrapers"):
            in_scrapers = False
            scrapers_indent = -1
        if match and match.group("key") == "scrapers":
            in_scrapers = True
            scrapers_indent = len(match.group("indent"))

    if in_scrapers:
        match = process_re.match(line.rstrip("\n"))
        if match and len(match.group("indent")) > scrapers_indent:
            found = True
            changed = True
            process_indent = len(match.group("indent"))
            skip_process = True
            if not inserted_comment:
                out.append(f'{match.group("indent")}# process scraper disabled: user-service collector cannot read root-owned PIDs without journal spam\n')
                inserted_comment = True
            continue

    out.append(line)

if not found:
    raise SystemExit("ERROR: no hostmetrics scrapers.process entry found; config left unchanged")
if not changed:
    raise SystemExit("ERROR: no changes made")

path.write_text("".join(out))
PY

  systemctl --user restart "$unit"
  sleep 2
  systemctl --user is-active --quiet "$unit"

  echo "Patched $config"
  echo "Backup: $backup"
  echo "Current $unit status: $(systemctl --user is-active "$unit")"
  echo "Recent permission-denied lines after restart:"
  journalctl --user -u "$unit" --since '2 minutes ago' --no-pager 2>/dev/null | grep -i 'permission denied' | tail -5 || echo "none"
}

case "${1:-}" in
  --apply-local)
    apply_local
    ;;
  -h|--help)
    cat <<'EOF'
Usage:
  scripts/fix-ghost-otel-process-noise.sh [user@host]
  scripts/fix-ghost-otel-process-noise.sh --apply-local

Environment:
  QA_HOST              remote host when no positional host is supplied
  OTELCOL_USER_UNIT    systemd user unit name (default: otelcol-agent.service)
EOF
    ;;
  *)
    host=${1:-${QA_HOST:-jorge@192.168.1.102}}
    unit=${OTELCOL_USER_UNIT:-otelcol-agent.service}
    ssh_opts=(
      -o StrictHostKeyChecking=no
      -o UserKnownHostsFile=/dev/null
      -o LogLevel=ERROR
    )
    ssh "${ssh_opts[@]}" "$host" "OTELCOL_USER_UNIT=$(printf %q "$unit") bash -s -- --apply-local" < "$0"
    ;;
esac
