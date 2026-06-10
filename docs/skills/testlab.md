# Testlab — knuckle Local VM & ISO Testing

Run knuckle interactively or headlessly inside a real Flatcar Container Linux VM.

## Quick Start

```bash
cd ~/src/knuckle

just vm           # interactive TUI — real install → auto-boots installed system
just vm-e2e       # automated 4-pass: DHCP · static · sysext · NVIDIA
just boot-iso     # build ISO → boot in QEMU serial console (Ctrl-a x to quit)
just e2e          # build ISO → launch interactive VM in Ghostty
just headless-test  # no VM — config generation only (runs anywhere)
just stop         # kill running VM
just clean        # kill VM + remove all artifacts
```

## How `just vm` Works

1. Builds binary (`linux/amd64`, CGO_ENABLED=0)
2. Creates qcow2 overlay on cached Flatcar base image (instant)
3. Creates 20G target disk
4. Boots QEMU with port-forward (2222→22)
5. Waits for SSH (~6s with KVM)
6. SCPs binary to `/tmp/knuckle`
7. SSHes in — runs knuckle interactively (real install, writes to /dev/vdb)
8. After install: kills installer VM and boots the installed target disk

**Antipattern:** Never embed the binary into Ignition via base64 (19 MB → 26 MB JSON).

## How `just vm-e2e` Works

Runs 4 automated passes back-to-back. No user interaction required.

| Pass | What it tests | Timeout |
|---|---|---|
| DHCP | Hostname, update strategy, locksmith | 15m |
| Static | `/etc/systemd/network/10-static.network` | 15m |
| Sysext | docker.raw present, `docker version` exits 0 | 25m |
| NVIDIA | NVIDIA driver sysext config, enabled-sysext.conf | 15m |

Each pass builds a fresh qcow2 overlay — passes are independent.

## How `just e2e` / `just boot-iso` Work

1. Builds ISO (if not already present in `output/`)
2. Opens Ghostty window with QEMU UEFI VM booting from ISO
3. GRUB menu appears (3s timeout), boots Flatcar
4. knuckle auto-launches on tty1 via systemd unit
5. User completes install interactively

After install, run `just boot-target` to boot the installed system.

## ISO Architecture

- **Kernel:** `flatcar_production_pxe.vmlinuz` (Flatcar CDN)
- **Initrd:** `flatcar_production_pxe_image.cpio.gz` + knuckle overlay cpio
- **Boot:** GRUB standalone EFI (`grub-mkstandalone`)
- **Assembly:** xorriso with El Torito EFI boot image
- **Overlay:** `/opt/knuckle` binary + `knuckle-installer.service`
- **UEFI only** (no BIOS/legacy)

Build deps: `x86_64-elf-grub-mkstandalone`, `xorriso`, `mtools`, `cpio`

## Post-Install Verification

```bash
# just vm boots the installed system automatically after knuckle exits.
# For vm-e2e, pass output shows SSH verification results.
# For ISO installs, reboot from knuckle's done screen, then:
ssh -p 2222 core@127.0.0.1 -o StrictHostKeyChecking=no \
  "hostname && uname -r && cat /etc/flatcar/update.conf"
```

## Key Facts

| Item | Value |
|---|---|
| Local VM SSH | `ssh -p 2222 core@127.0.0.1` |
| Target disk in VM | `/dev/vdb` (20G virtio) |
| Image format | qcow2 overlay (backing: `.vm/flatcar_base_amd64.img`) |
| Boot time (KVM) | ~6s to SSH |
| SSH options | `-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR` |

## Agent Limitations

**Agents CANNOT verify TUI interactive behavior.**

| Can verify | Cannot verify |
|---|---|
| Binary builds | Forms render correctly |
| Unit tests pass | User can navigate steps |
| VM boots (SSH works) | Install progress animates |
| Installed system config | TUI doesn't crash mid-flow |
| Headless mode output | Interactive experience |

Correct protocol: launch a Ghostty terminal for the user, say "launched — awaiting feedback", wait.

```bash
ghostty --gtk-single-instance=false -e bash -c "cd ~/src/knuckle && just vm ''" &
```

## Remote Testing on Ghost

`just vm-e2e` and `just headless-test` are ghost-safe (headless, no display).
`just vm`, `just e2e`, `just boot-iso` require local display — **never run these on ghost**.

QEMU port-forward binds `127.0.0.1:2222`. To SSH into a VM running on ghost:
```bash
ssh jorge@ghost   # then from ghost:
ssh -p 2222 core@127.0.0.1
```
Never `ssh -p 2222 core@jorge@ghost` — that reaches ghost's sshd.

## Gotchas

| Problem | Fix |
|---|---|
| Ghostty window invisible | `--gtk-single-instance=false` |
| ISO doesn't boot (EFI shell) | Need OVMF firmware (`-drive if=pflash,...`) |
| GRUB "file not found" | Needs `search --file /vmlinuz --set=root` in grub.cfg |
| VM port 2222 in use | `just stop` |
| Base image missing | First `just vm` downloads ~470 MB Flatcar image (cached in `.vm/`) |
| `just e2e` fails on ghost | Uses `-display gtk` — local display required |
