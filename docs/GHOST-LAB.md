# QA Lab Setup

`just qa-pr <PR>` runs a full Flatcar VM install + boot + domain assertions
for any PR. It works on a laptop, on a dedicated test machine, or remotely.

---

## What it needs

- Linux host with KVM (`/dev/kvm` accessible)
- QEMU (`qemu-system-x86_64`)
- A Flatcar QEMU base image (~477 MB)
- `just`, `gh` CLI, standard Go toolchain (for the build step)

---

## Laptop / local setup (simplest)

```bash
# 1. Download the Flatcar QEMU base image (~477 MB, one-time)
mkdir -p /var/tmp/knuckle-test
curl -L https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_qemu_image.img.bz2 \
  | bunzip2 > /var/tmp/knuckle-test/flatcar_base.img

# 2. Run a PR test (defaults to localhost)
just qa-pr 170

# Artifacts land in .qa/runs/pr-170-TIMESTAMP/
```

That's it. No additional configuration needed.

---

## Remote machine (Jorge's setup — ghost at 192.168.1.102)

The base image and QEMU run on ghost. The build and unit tests still run
locally; only the VM portion runs remotely.

```bash
# Set once in your shell profile or .env:
export QA_HOST=jorge@192.168.1.102
export QA_FLATCAR_BASE=/var/tmp/knuckle-test/flatcar_base.img

# Run exactly the same way:
just qa-pr 170
```

The script SSH-tunnels into the remote host for all VM operations.
`QA_FLATCAR_BASE` must be a path on `QA_HOST`, not on your local machine.

---

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `QA_HOST` | `localhost` | Machine where QEMU runs (`user@host` or `localhost`) |
| `QA_FLATCAR_BASE` | `/var/tmp/knuckle-test/flatcar_base.img` | Path to Flatcar QEMU image **on QA_HOST** |
| `FILE_ISSUES` | `0` | Set to `1` to auto-file GitHub issues on failure |

---

## What the test does

For any PR, `just qa-pr <N>` runs in order:

1. **Build** the binary from the PR's head commit
2. **`just ci`** — unit tests, lint, coverage gate (on local machine)
3. **Boot Flatcar installer VM** on QA_HOST (fresh qcow2 overlay)
4. **Tool check** — sfdisk version, wipefs version, --relocate present
5. **Headless --dry-run** — config generation + Ignition compile, no disk writes
6. **Real headless install** — flatcar-install writes to /dev/vdb
7. **Boot the installed system** — kills installer VM, boots target disk
8. **Domain assertions** — quoted evidence from inside the booted installed Flatcar

Steps 3-8 run on `QA_HOST` via SSH. The assertion script (`assert.sh`) is
generated locally and SCPed to QA_HOST, then into the VM — no heredoc escaping.

---

## Ghost observability noise

Ghost may run `otelcol-agent` as a systemd user service for lab telemetry. Do
not enable the hostmetrics `process` scraper there: as an unprivileged user it
tries to inspect root-owned PIDs and can emit huge permission-denied batches to
`journalctl --user -u otelcol-agent` every collection interval. Keep aggregate
process counts via the `processes` scraper instead.

To apply the repository-maintained fix to ghost (or any `QA_HOST`):

```bash
./scripts/fix-ghost-otel-process-noise.sh              # defaults to jorge@192.168.1.102
QA_HOST=user@host ./scripts/fix-ghost-otel-process-noise.sh
./scripts/fix-ghost-otel-process-noise.sh --apply-local # run directly on ghost
```

The script backs up the collector config, removes only the `scrapers.process`
block, restarts the user service, and reports any recent permission-denied
journal lines.

---

## Artifacts

Each run saves everything to `.qa/runs/pr-N-TIMESTAMP/`:

```
.qa/runs/pr-170-20260522-193000/
├── report.md              # the full test report (publish to PR with gh pr comment)
├── build.log              # go build output
├── ci.log                 # just ci output
├── knuckle-install.log    # knuckle slog output from inside the VM
└── ghost/                 # all QEMU artifacts fetched from QA_HOST:
    ├── serial-installer.log
    ├── serial-installed.log
    └── ...
```

Failed runs also write `issue-body.md` ready to file:

```bash
gh issue create --repo projectbluefin/knuckle \
  --title "qa: PR #170 — <summary>" \
  --body-file .qa/runs/pr-170-.../issue-body.md
```

Or set `FILE_ISSUES=1` to file automatically.

---

## Minimum host requirements

| Resource | Minimum | Notes |
|---|---|---|
| RAM | 4 GB free | 2 GB for installer VM + 2 GB for installed VM |
| Disk | 5 GB free | ~500 MB install + logs |
| CPU | KVM capable | Software emulation (TCG) works but takes ~5× longer |
| Ports | 2300–2315 free | Script auto-allocates; adjust if you have conflicts |

On a laptop with 8 GB RAM and SSD, a full Tier 3 run takes ~8 minutes.
On ghost (32 cores, NVMe), ~3 minutes.

---

## Adding a new QA host

Set two env vars and run:

```bash
# On the new host: download the base image once
ssh user@newhost "
  mkdir -p /var/tmp/knuckle-test
  curl -L https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_qemu_image.img.bz2 \
    | bunzip2 > /var/tmp/knuckle-test/flatcar_base.img
"

# On your dev machine:
export QA_HOST=user@newhost
just qa-pr 170
```

The only requirement on the remote host is QEMU + KVM + SSH access.

---

## KubeVirt on Ghost (installed 2026-05-23)

Ghost runs k3s v1.32.4 + KubeVirt v1.8.2. This provides a second testing
path for `domain:tui` PRs: a persistent Flatcar VM accessible via SSH,
without the QEMU `hostfwd` port-forwarding complexity.

The QEMU `qa-test-pr.sh` path remains the primary gate for all Tier 1/3 tests.
KubeVirt is used for interactive TUI verification — the human connects and looks.

### When to use which approach

| Approach | Use for |
|---|---|
| `qa-test-pr.sh` | All automated Tier 0/1/3 tests, merge-gate evidence |
| KubeVirt VM | Interactive TUI visual verification (`domain:tui` PRs) |

### Quick reference

```bash
# Check health
ssh jorge@192.168.1.102 "kubectl -n kubevirt get pods --no-headers | grep -c Running"
# Expected: 5

# List running VMs
ssh jorge@192.168.1.102 "kubectl -n knuckle-test get vmi"

# Create a TUI test VM for PR N (see full procedure in ghost-testlab skill)
# 1. Convert flatcar_base.img to raw, inject SSH key, apply VM manifest
# 2. Deploy binary:  scp /tmp/knuckle-prN jorge@ghost:/tmp/
# 3. Ghost→VM:  ssh jorge@ghost "scp ... core@<VMIP>:/tmp/knuckle"
# 4. Interactive: ssh -J jorge@192.168.1.102 core@<VMIP> -t /tmp/knuckle

# Stop and delete a VM when done
ssh jorge@192.168.1.102 "virtctl stop flatcar-pr<N> -n knuckle-test"
ssh jorge@192.168.1.102 "kubectl -n knuckle-test delete vm flatcar-pr<N>"
```

### Disk files (hostDisk requirements)

KubeVirt's `hostDisk` has strict requirements:

```bash
# Files MUST be:
# - raw format (not qcow2) — KubeVirt passes type=raw to QEMU
# - owned by qemu:qemu (uid 107)
# - SELinux context: container_file_t
sudo qemu-img convert -p -f qcow2 -O raw flatcar_base.img flatcar-prN-raw.img
sudo chown qemu:qemu flatcar-prN-raw.img && sudo chmod 664 flatcar-prN-raw.img
sudo chcon -t container_file_t flatcar-prN-raw.img
```

### SSH key injection (cloudInitNoCloud does not work for Flatcar)

Flatcar on QEMU reads ignition from `fw_cfg`, not from a NoCloud ISO.
The only way to inject SSH keys into a KubeVirt Flatcar VM is to mount
the raw disk and write `authorized_keys` directly before first boot:

```bash
# Flatcar ROOT = partition 9, sector offset 12722176
# Offset bytes: 12722176 × 512 = 6513754112
virtctl stop flatcar-prN -n knuckle-test
ssh jorge@192.168.1.102 "
  sudo mount -o loop,offset=6513754112 /var/tmp/knuckle-test/flatcar-prN-raw.img /mnt/flatcar-root
  sudo mkdir -p /mnt/flatcar-root/home/core/.ssh
  echo '<your-pubkey>' | sudo tee /mnt/flatcar-root/home/core/.ssh/authorized_keys
  sudo chown -R 500:500 /mnt/flatcar-root/home/core/.ssh  # core UID=500
  sudo chmod 700 /mnt/flatcar-root/home/core/.ssh
  sudo chmod 600 /mnt/flatcar-root/home/core/.ssh/authorized_keys
  sudo umount /mnt/flatcar-root
"
virtctl start flatcar-prN -n knuckle-test
```

