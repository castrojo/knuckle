# Release — knuckle

> Reference: [`RELEASE.md`](../RELEASE.md) (gates checklist), [`CI-AND-TESTING.md`](../CI-AND-TESTING.md)

Run every step in order. No shortcuts. All 5 gates must be green before tagging.

## Asset Inventory (expected: 16 total)

Each release ships **8 × amd64 + 8 × arm64**:

| Asset | ×amd64 | ×arm64 |
|---|---|---|
| `knuckle-linux-<arch>` | ✅ | ✅ |
| `knuckle-linux-<arch>.sha256` | ✅ | ✅ |
| `knuckle-linux-<arch>.spdx.json` | ✅ | ✅ |
| `knuckle-linux-<arch>.spdx.json.bundle` | ✅ | ✅ |
| `knuckle-installer-stable-<arch>.iso` | ✅ | ✅ |
| `knuckle-installer-stable-<arch>.iso.sha256` | ✅ | ✅ |
| `knuckle-installer-stable-<arch>.iso.bundle` | ✅ | ✅ |
| `knuckle-linux-<arch>.bundle` | ✅ | ✅ |

## Pre-Release E2E Gate

### Step 1 — CI gate

```bash
cd ~/src/knuckle
git checkout main && git pull upstream main
just ci   # must exit 0
```

`cmd/knuckle` TTY tests failing (issue #512) do NOT block release. All other failures block.

### Step 2 — Release preflight

```bash
just release-preflight   # just ci + sysext catalog coverage + NVIDIA driver series check
```

Must exit 0.

### Step 3 — VM e2e (all 4 passes)

```bash
just vm-e2e   # DHCP → static → sysext → NVIDIA
```

All 4 passes must exit 0. Any FAIL blocks the release.

| Pass | Key assertion |
|---|---|
| DHCP | hostname, locksmith enabled |
| Static | `/etc/systemd/network/10-static.network` |
| Sysext | `/etc/extensions/docker.raw` present, `docker version` |
| NVIDIA | `/etc/sysupdate.d/nv-*.conf` present |

### Step 4 — amd64 ISO smoke

```bash
just build
just iso stable   # → output/knuckle-installer-stable-amd64.iso
OVMF=$(ls /usr/share/OVMF/OVMF_CODE*.fd /usr/share/edk2/ovmf/OVMF_CODE*.fd 2>/dev/null | head -1)
just iso-smoke output/knuckle-installer-stable-amd64.iso "$OVMF" 120
```

Pass criteria: `initrd-root-device.target` + `initrd-usr-fs.target` + `getty.target`
in serial log; zero dracut errors; `systemd.gpt_auto=0` on both BLS entries.

> ⚠️ **Flatcar 4230.x regression (issue #737):** `initrd-usr-fs.target` may not appear
> in serial log even though boot is healthy. Non-required; does not block release.

### Step 5 — arm64 ISO smoke (manual, TCG)

No native KVM for arm64 — uses software emulation. Too slow for CI; run manually before tagging.

```bash
KNUCKLE_ARCH=arm64 just build
KNUCKLE_ARCH=arm64 just iso stable
AAVMF=$(ls /usr/share/AAVMF/AAVMF_CODE.fd /usr/share/qemu-efi-aarch64/QEMU_EFI.fd 2>/dev/null | head -1)
timeout 300 qemu-system-aarch64 \
  -M virt -cpu cortex-a57 -m 2048 \
  -drive if=pflash,format=raw,readonly=on,file="$AAVMF" \
  -cdrom output/knuckle-installer-stable-arm64.iso \
  -drive if=virtio,file=/tmp/arm64-smoke-target.img,format=raw \
  -nographic 2>&1 | tee /tmp/arm64-iso-smoke.log || true
grep -E "initrd-root-device|initrd-usr-fs|getty.target" /tmp/arm64-iso-smoke.log
grep -c "xd2root\|x2dauto\|dracut.*skip" /tmp/arm64-iso-smoke.log   # must be 0
```

Quoted serial log evidence must be pasted into the release epic issue before tagging.

## Tag & Publish

```bash
git status        # must be clean
git log --oneline -5
git tag v0.X.Y
git push upstream v0.X.Y   # triggers release.yml: create-release → [amd64 | arm64] → publish
gh run watch --repo projectbluefin/knuckle
```

The `publish` job gates on both arch jobs completing. **Do NOT push the same tag twice.**

## Post-Release Verification

```bash
# Asset count (must be 16)
gh release view vX.Y.Z --repo projectbluefin/knuckle --json assets \
  --jq '.assets | length'

# Both arch ISOs present
gh release view vX.Y.Z --repo projectbluefin/knuckle --json assets \
  --jq '.assets | map(.name) | map(select(test("arm64|amd64"))) | sort | .[]'

# Cosign bundles (must be 4: binary + iso per arch)
gh release view vX.Y.Z --repo projectbluefin/knuckle --json assets \
  --jq '.assets | map(select(.name | endswith(".bundle"))) | length'
```

## Known Failure Modes

| Symptom | Cause | Fix |
|---|---|---|
| arm64 assets missing | Race in parallel arch jobs | Ensure `release.yml` uses parallel create-release gate (PR #606 must be merged) |
| `Cannot upload asset … to an immutable release` | Tag pushed twice | Delete release + tag, fix, re-tag |
| `iso-smoke` hangs forever | Missing `systemd.gpt_auto=0` | Check `scripts/build-iso.sh` — both BLS entries must have it |
| arm64 cross-compile fails | `CGO_ENABLED` not 0 | Check `CGO_ENABLED=0` in arm64 build steps |
| Workflow run blocked | First-time contributor | Approve via `gh api .../actions/runs/<ID>/approve --method POST` |
| OSSF Scorecard skipped | Only runs on push to `main`, not tags | Expected — not a blocker |

## Lessons Learned

### arm64 artifacts missing — v0.7.0 (2026-05-28)

Root cause: `release-arm64` had `needs: release-amd64`. On tag retry, first run's
`publish` made the release immutable before arm64 could upload. Fixed in PR #606
(parallel `create-release` gate). Never tag before PR #606 is merged.

### Always run arm64 ISO smoke manually

TCG is ~5× real-time — too slow for CI. Ghost has no native arm64 KVM. Cross-compile
succeeds but iso must be smoke-tested manually. TCG boot takes ~5 min. Evidence must be
in the release epic.

### `just vm-e2e` does NOT test ISO boot

`vm-e2e` deploys via headless mode, not from the installer ISO. Use `just iso-smoke` or
`just boot-iso` for ISO boot validation. Both are required for a complete release gate.
