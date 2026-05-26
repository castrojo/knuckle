#!/usr/bin/env bash
# nvidia_check.sh — Verify model.go NvidiaDriverOptions against the Flatcar docs.
#
# The Flatcar NVIDIA driver series list is maintained by the Flatcar project
# and updated independently of the sysext bakery. This script fetches the
# official Flatcar NVIDIA customization guide from GitHub and reports which
# driver series are mentioned there vs what is hardcoded in model.go.
#
# There is no machine-readable enumeration API for Flatcar-official driver
# sysexts; the docs are the authoritative reference.
#
# Usage: ./scripts/nvidia_check.sh
#        Run manually or via: just nvidia-check
set -euo pipefail

DOCS_URL="https://api.github.com/repos/flatcar/flatcar-website/contents/content/docs/latest/setup/customization/using-nvidia.md"
MODEL_GO="internal/model/model.go"

echo "nvidia_check — verifying model.go NvidiaDriverOptions against Flatcar docs"
echo "──────────────────────────────────────────────────────────────────────────"
echo ""

# ── Fetch Flatcar docs ────────────────────────────────────────────────────────
echo "Fetching Flatcar NVIDIA docs..."
if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub." >&2
  echo "  Check network or try: curl -sf '$DOCS_URL'" >&2
  exit 2
fi

DOC_CONTENT=$(curl -sf "$DOCS_URL" | python3 -c "
import sys, json, base64
data = json.load(sys.stdin)
print(base64.b64decode(data['content']).decode())
" 2>/dev/null) || {
  echo "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub." >&2
  echo "  Check network or try: curl -sf '$DOCS_URL'" >&2
  exit 2
}

# Extract all nvidia-drivers-* patterns mentioned in the docs.
DOC_SERIES=$(printf '%s\n' "$DOC_CONTENT" | grep -oE 'nvidia-drivers-[a-z0-9-]+' | sort -u || true)

echo ""
echo "Driver series mentioned in Flatcar NVIDIA docs:"
if [[ -z "$DOC_SERIES" ]]; then
  echo "  (none found — docs may have changed structure)"
else
  while IFS= read -r series; do
    id="${series#nvidia-drivers-}"
    echo "  $id  ($series)"
  done <<< "$DOC_SERIES"
fi

# ── Extract model.go entries ──────────────────────────────────────────────────
echo ""
echo "Driver series in model.go NvidiaDriverOptions:"
MODEL_IDS=$(awk '/NvidiaDriverOptions/,/^\}/' "$MODEL_GO" | \
  grep -oE 'ID:\s+"[^"]+"' | \
  sed 's/.*ID:\s*"\([^"]*\)".*/\1/' | sort -u)

while IFS= read -r id; do
  echo "  $id  (nvidia-drivers-$id)"
done <<< "$MODEL_IDS"

# ── Compare ───────────────────────────────────────────────────────────────────
echo ""
echo "──────────────────────────────────────────────────────────────────────────"

# Warn if docs mention a series not in model.go
NEEDS_ADD=0
if [[ -n "$DOC_SERIES" ]]; then
  while IFS= read -r series; do
    id="${series#nvidia-drivers-}"
    if ! echo "$MODEL_IDS" | grep -q "^${id}$"; then
      echo "⚠ MISSING IN MODEL: $id — mentioned in Flatcar docs but not in model.go"
      NEEDS_ADD=1
    fi
  done <<< "$DOC_SERIES"
fi

# Warn about series in model.go that docs don't mention (not necessarily wrong —
# docs may just show the latest example).
while IFS= read -r id; do
  if [[ -n "$DOC_SERIES" ]] && ! echo "$DOC_SERIES" | grep -q "nvidia-drivers-${id}$"; then
    echo "  NOTE: $id is in model.go but not mentioned in current Flatcar docs"
    echo "        (This is normal — docs typically only show the recommended series)"
  fi
done <<< "$MODEL_IDS"

echo ""
if [[ $NEEDS_ADD -eq 1 ]]; then
  echo "ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions"
  echo ""
  echo "  1. Add the missing series to NvidiaDriverOptions in internal/model/model.go"
  echo "  2. Set Recommended: true on the newest open-source series"
  echo "  3. Update DefaultNvidiaDriverSeries to the newest recommended series"
  echo "  4. Update Description field with GPU compatibility information"
  echo "  5. Update the NVIDIA section in docs/SYSEXTS.md"
  echo "  6. Run: just ci"
  exit 1
else
  echo "✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs."
  echo ""
  echo "Note: The Flatcar docs may only show a single example series."
  echo "For authoritative driver series availability, check:"
  echo "  https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/"
  echo "  https://github.com/flatcar/flatcar-website"
fi
