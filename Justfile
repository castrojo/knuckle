# Knuckle — Flatcar Container Linux TUI Installer
# https://github.com/castrojo/knuckle

QEMU := if path_exists("/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64") == "true" { "/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64" } else { "/usr/bin/qemu-system-x86_64" }

default:
    @just --list

# Build and boot installer VM — knuckle launches automatically on serial console
vm:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building knuckle..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/knuckle-linux-amd64 ./cmd/knuckle

    mkdir -p .vm
    if [ ! -f ".vm/flatcar_base.img" ]; then
        echo "Downloading Flatcar stable QEMU image..."
        curl -L -o .vm/flatcar_base.img.bz2 \
            "https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_qemu_image.img.bz2"
        bunzip2 .vm/flatcar_base.img.bz2
    fi

    # Kill any running VM first
    if [ -f .vm/qemu.pid ]; then
        kill "$(cat .vm/qemu.pid)" 2>/dev/null || true
        rm -f .vm/qemu.pid
        sleep 1
    fi

    # Fresh disks every boot (Ignition only runs on first boot)
    rm -f .vm/boot.img .vm/target.qcow2
    cp .vm/flatcar_base.img .vm/boot.img
    qemu-img create -f qcow2 .vm/target.qcow2 20G

    # Tiny Ignition: SSH key + autologin runs knuckle on serial console
    SSH_KEY=$(cat ~/.ssh/id_ed25519.pub)
    printf '{"ignition":{"version":"3.3.0"},"passwd":{"users":[{"name":"core","sshAuthorizedKeys":["%s"]}]},"systemd":{"units":[{"name":"sshd.service","enabled":true},{"name":"serial-getty@ttyS0.service","dropins":[{"name":"autologin.conf","contents":"[Service]\\nExecStart=\\nExecStart=-/sbin/agetty --autologin core --noclear %%I 115200 linux"}]}]}}' "$SSH_KEY" > .vm/config.ign

    echo "Booting VM (waiting for SSH)..."
    {{QEMU}} \
        -m 2048 -smp 2 -enable-kvm \
        -drive if=virtio,file=.vm/boot.img,format=qcow2 \
        -drive if=virtio,file=.vm/target.qcow2,format=qcow2 \
        -fw_cfg name=opt/org.flatcar-linux/config,file=.vm/config.ign \
        -net nic,model=virtio -net user,hostfwd=tcp::2222-:22 \
        -display none -daemonize -serial file:.vm/serial.log \
        -pidfile .vm/qemu.pid

    # Wait for SSH
    for i in $(seq 1 30); do
        if ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=2 -p 2222 core@127.0.0.1 true 2>/dev/null; then
            break
        fi
        sleep 2
    done

    echo "Deploying knuckle..."
    scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P 2222 \
        bin/knuckle-linux-amd64 core@127.0.0.1:/tmp/knuckle 2>/dev/null

    echo "Launching installer..."
    exec ssh -t -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 \
        core@127.0.0.1 '/tmp/knuckle --dry-run'

# SSH into the running VM
ssh:
    ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 core@127.0.0.1

# Run tests
test:
    go test ./...

# Full CI (lint + test + build)
ci:
    go mod tidy
    golangci-lint run ./...
    go test -race ./...
    go build -o bin/knuckle ./cmd/knuckle

# Clean everything
clean:
    #!/usr/bin/env bash
    if [ -f .vm/qemu.pid ]; then kill "$(cat .vm/qemu.pid)" 2>/dev/null || true; fi
    rm -rf bin/ .vm/
