#!/usr/bin/env bash
# Build a bootable FCOS live ISO containing the knuckle installer.
#
# Uses coreos-installer to:
#  1. Download the FCOS live ISO for the target stream + architecture
#  2. Customise the live image: inject the knuckle binary (as a base64 Ignition
#     file) and enable the knuckle-installer systemd service via --live-ignition
#
# The resulting ISO boots directly into a live FCOS environment where knuckle
# runs on tty1.  The Conflicts=getty@tty1.service directive in the service unit
# prevents the default FCOS autologin getty from competing with the TUI.
#
# Requirements: coreos-installer, jq (optional — for ignition pretty-print)
#   Install (Fedora): sudo dnf install -y coreos-installer   (or: just tools-fcos)
#   Non-Fedora: run the digest-pinned quay.io/coreos/coreos-installer:release container
#
# Usage: ./scripts/build-fcos-iso.sh [--stream stable|testing|next] [--arch amd64|arm64] [--binary /path/to/knuckle] [--ssh-key "ssh-ed25519 ..."]
set -euo pipefail

# ── Argument parsing ─────────────────────────────────────────────────────────
STREAM="stable"
ARCH="amd64"
BINARY_OVERRIDE=""
SSH_PUB_KEY=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --stream=*) STREAM="${1#--stream=}"; shift ;;
        --stream)   STREAM="$2"; shift 2 ;;
        --arch=*)   ARCH="${1#--arch=}"; shift ;;
        --arch)     ARCH="$2"; shift 2 ;;
        --binary=*) BINARY_OVERRIDE="${1#--binary=}"; shift ;;
        --binary)   BINARY_OVERRIDE="$2"; shift 2 ;;
        --ssh-key=*) SSH_PUB_KEY="${1#--ssh-key=}"; shift ;;
        --ssh-key)  SSH_PUB_KEY="$2"; shift 2 ;;
        stable|testing|next) STREAM="$1"; shift ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

# ── Validate arguments ───────────────────────────────────────────────────────
case "$STREAM" in
    stable|testing|next) ;;
    *) echo "error: --stream must be stable, testing, or next (got '$STREAM')" >&2; exit 1 ;;
esac
case "$ARCH" in
    amd64|arm64) ;;
    *) echo "error: --arch must be amd64 or arm64 (got '$ARCH')" >&2; exit 1 ;;
esac

# coreos-installer uses "x86_64" and "aarch64" internally
case "$ARCH" in
    amd64)  COREOS_ARCH="x86_64" ;;
    arm64)  COREOS_ARCH="aarch64" ;;
esac

# ── Dependency check ─────────────────────────────────────────────────────────
if ! command -v coreos-installer &>/dev/null; then
    echo "error: coreos-installer not found" >&2
    echo "  Install (Fedora): sudo dnf install -y coreos-installer" >&2
    echo "  Or run:  just tools-fcos" >&2
    exit 1
fi

# ── Paths ────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$ROOT_DIR/.fcos-iso-build"
OUTPUT_DIR="$ROOT_DIR/output"

BINARY=""
if [[ -n "$BINARY_OVERRIDE" ]]; then
    BINARY="$BINARY_OVERRIDE"
elif [[ -f "$ROOT_DIR/bin/knuckle-${ARCH}" ]]; then
    BINARY="$ROOT_DIR/bin/knuckle-${ARCH}"
elif [[ -f "$ROOT_DIR/bin/knuckle" ]]; then
    BINARY="$ROOT_DIR/bin/knuckle"
elif [[ -f "$ROOT_DIR/knuckle" ]]; then
    BINARY="$ROOT_DIR/knuckle"
fi

echo "=== Building knuckle FCOS installer ISO (stream: $STREAM, arch: $ARCH) ==="

# ── 1. Build knuckle binary ───────────────────────────────────────────────────
if [[ ! -f "$BINARY" ]]; then
    echo "[1/4] Building knuckle ($ARCH)..."
    VERSION="$(git -C "$ROOT_DIR" describe --tags --always 2>/dev/null || echo dev)"
    (cd "$ROOT_DIR" && GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 \
        go build -ldflags="-s -w -X main.version=${VERSION}" -o "bin/knuckle-${ARCH}" ./cmd/knuckle)
    BINARY="$ROOT_DIR/bin/knuckle-${ARCH}"
else
    echo "[1/4] Using existing knuckle binary: $BINARY"
fi
echo "  binary : $(du -h "$BINARY" | cut -f1)"

# ── 2. Download FCOS live ISO ─────────────────────────────────────────────────
mkdir -p "$BUILD_DIR"
echo "[2/4] Downloading FCOS live ISO (stream: $STREAM, arch: $COREOS_ARCH)..."

# coreos-installer download writes a versioned filename; find it after download.
# Use a hash of stream+arch as a cache key — re-download if not present.
ISO_CACHE_DIR="$BUILD_DIR/iso-cache-${STREAM}-${ARCH}"
mkdir -p "$ISO_CACHE_DIR"

LIVE_ISO=""
# Check for a previously downloaded ISO
EXISTING_ISO="$(ls "$ISO_CACHE_DIR"/*.iso 2>/dev/null | head -1 || true)"
if [[ -n "$EXISTING_ISO" ]]; then
    echo "  Using cached FCOS ISO: $(basename "$EXISTING_ISO")"
    LIVE_ISO="$EXISTING_ISO"
else
    echo "  Fetching from builds.coreos.fedoraproject.org..."
    coreos-installer download \
        --stream "$STREAM" \
        --platform metal \
        --format iso \
        --architecture "$COREOS_ARCH" \
        --directory "$ISO_CACHE_DIR"
    LIVE_ISO="$(ls "$ISO_CACHE_DIR"/*.iso | head -1)"
    echo "  Downloaded: $(basename "$LIVE_ISO") ($(du -h "$LIVE_ISO" | cut -f1))"
fi

# ── 3. Generate live Ignition config ─────────────────────────────────────────
# The Ignition config is applied in the LIVE FCOS environment (--live-ignition),
# not on the installed system.  It:
#   - Writes the knuckle binary to /opt/knuckle (base64-encoded)
#   - Enables sshd for debug access
#   - Installs + enables knuckle-installer.service (with tty1 conflict fix)
echo "[3/4] Generating live Ignition config..."

IGN_FILE="$BUILD_DIR/live-ignition.ign"

# Encode the binary as a base64 data URL for the Ignition storage.files entry.
BINARY_B64="$(base64 -w0 "$BINARY")"
BINARY_SIZE="$(stat -c%s "$BINARY")"

# Build passwd section if an SSH key was provided
PASSWD_JSON="null"
if [[ -n "$SSH_PUB_KEY" ]]; then
    PASSWD_JSON="{\"users\":[{\"name\":\"core\",\"sshAuthorizedKeys\":[$(printf '%s' "$SSH_PUB_KEY" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')]}"
    PASSWD_JSON="${PASSWD_JSON}]}"
fi

# Write the Ignition 3.3.0 JSON.
# The service unit includes Conflicts=getty@tty1.service because FCOS live
# images autologin the "core" user on tty1 by default.
python3 - "$IGN_FILE" "$BINARY_B64" "$BINARY_SIZE" "$PASSWD_JSON" <<'PYEOF'
import json, sys, os

ign_file   = sys.argv[1]
binary_b64 = sys.argv[2]
binary_size = int(sys.argv[3])
passwd_raw  = sys.argv[4]

service_unit = """\
[Unit]
Description=Knuckle FCOS Installer
After=multi-user.target
Conflicts=getty@tty1.service
Before=getty@tty1.service
ConditionPathExists=/opt/knuckle

[Service]
Type=idle
ExecStart=/opt/knuckle
StandardInput=tty
StandardOutput=tty
TTYPath=/dev/tty1
TTYReset=yes
TTYVHangup=yes
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target"""

enabled = True

config = {
    "ignition": {"version": "3.3.0"},
    "storage": {
        "files": [
            {
                "path": "/opt/knuckle",
                "mode": 0o755,
                "contents": {
                    "source": f"data:;base64,{binary_b64}",
                    "verification": {}
                }
            }
        ]
    },
    "systemd": {
        "units": [
            {"name": "sshd.service", "enabled": True},
            {
                "name": "knuckle-installer.service",
                "enabled": True,
                "contents": service_unit
            }
        ]
    }
}

if passwd_raw != "null":
    config["passwd"] = json.loads(passwd_raw)

with open(ign_file, "w") as f:
    json.dump(config, f, indent=2)

print(f"  Ignition: {ign_file}")
print(f"  binary size: {binary_size} bytes embedded")
PYEOF

# ── 4. Customize FCOS ISO ─────────────────────────────────────────────────────
echo "[4/4] Customising FCOS live ISO with coreos-installer..."

mkdir -p "$OUTPUT_DIR"
ISO_OUT="$OUTPUT_DIR/knuckle-fcos-installer-${STREAM}-${ARCH}.iso"

coreos-installer iso customize \
    --live-ignition "$IGN_FILE" \
    --output "$ISO_OUT" \
    "$LIVE_ISO"

echo ""
echo "ISO built: $ISO_OUT ($(du -h "$ISO_OUT" | cut -f1))"
echo ""
echo "Test with QEMU (UEFI, amd64):"
echo "  OVMF=/usr/share/OVMF/OVMF_CODE.fd"
echo "  qemu-system-x86_64 -m 4096 -enable-kvm \\"
echo "    -drive if=pflash,format=raw,readonly=on,file=\$OVMF \\"
echo "    -cdrom $ISO_OUT \\"
echo "    -drive if=virtio,file=target.qcow2,format=qcow2 \\"
echo "    -nographic"
echo ""
echo "Write to USB:"
echo "  sudo dd if=$ISO_OUT of=/dev/sdX bs=4M status=progress"
