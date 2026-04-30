# Knuckle — Flatcar Container Linux TUI Installer
# https://github.com/castrojo/knuckle

QEMU := if path_exists("/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64") == "true" { "/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64" } else { "/usr/bin/qemu-system-x86_64" }
OVMF := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
ISO := ".vm/flatcar_production_iso_image.iso"
QEMU_IMG := ".vm/flatcar_production_qemu_image.img"
DISK := ".vm/test-disk.qcow2"

default:
    @just --list

# Format code
fmt:
    gofumpt -w .

# Run linter
lint:
    golangci-lint run ./...

# Run tests
test:
    go test ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Build binary
build:
    go build -o bin/knuckle ./cmd/knuckle

# Cross-compile for linux/amd64 (static)
build-linux:
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/knuckle-linux-amd64 ./cmd/knuckle

# Run the TUI locally (dry-run)
run:
    go run ./cmd/knuckle --dry-run

# Tidy dependencies
tidy:
    go mod tidy

# Run govulncheck
vuln:
    govulncheck ./...

# Full CI pipeline (tidy + lint + test + build)
ci: tidy lint test-race build

# Download Flatcar stable QEMU image for VM testing
download-image:
    mkdir -p .vm
    @if [ ! -f {{QEMU_IMG}} ]; then \
        echo "Downloading Flatcar stable QEMU image..."; \
        curl -L -o .vm/flatcar_production_qemu_image.img.bz2 \
            "https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_qemu_image.img.bz2"; \
        echo "Decompressing..."; \
        bunzip2 .vm/flatcar_production_qemu_image.img.bz2; \
    else \
        echo "QEMU image already exists: {{QEMU_IMG}}"; \
    fi

# Download Flatcar stable ISO (for install-to-disk testing)
download-iso:
    mkdir -p .vm
    @if [ ! -f {{ISO}} ]; then \
        echo "Downloading Flatcar stable ISO..."; \
        curl -L -o {{ISO}} "https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_iso_image.iso"; \
    else \
        echo "ISO already exists: {{ISO}}"; \
    fi

# Generate Ignition config for VM (SSH key-based login)
generate-ignition:
    mkdir -p .vm
    @echo '{"ignition":{"version":"3.3.0"},"passwd":{"users":[{"name":"core","sshAuthorizedKeys":["'$(cat ~/.ssh/id_ed25519.pub)'"]}]},"systemd":{"units":[{"name":"sshd.service","enabled":true}]}}' > .vm/config.ign

# Boot Flatcar QEMU VM (daemonized, SSH on port 2222)
vm: build-linux download-image generate-ignition
    {{QEMU}} \
        -m 2048 \
        -smp 2 \
        -enable-kvm \
        -drive if=virtio,file={{QEMU_IMG}},format=qcow2 \
        -fw_cfg name=opt/org.flatcar-linux/config,file=.vm/config.ign \
        -net nic,model=virtio -net user,hostfwd=tcp::2222-:22 \
        -serial file:.vm/serial.log \
        -display none -daemonize
    @echo "VM started. Waiting for boot..."
    @sleep 20
    @echo "Copying knuckle binary to VM..."
    scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P 2222 \
        bin/knuckle-linux-amd64 core@127.0.0.1:/tmp/knuckle
    @echo "VM ready! Run: just ssh"

# Boot Flatcar VM with serial console (interactive, foreground)
vm-console: build-linux download-image generate-ignition
    {{QEMU}} \
        -m 2048 \
        -smp 2 \
        -enable-kvm \
        -drive if=virtio,file={{QEMU_IMG}},format=qcow2 \
        -fw_cfg name=opt/org.flatcar-linux/config,file=.vm/config.ign \
        -net nic,model=virtio -net user,hostfwd=tcp::2222-:22 \
        -nographic

# SSH into running VM (default Flatcar user: core)
ssh:
    ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 core@127.0.0.1

# Run knuckle --dry-run inside the VM (requires 'just vm' first)
vm-test:
    ssh -t -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 \
        core@127.0.0.1 '/tmp/knuckle --dry-run'

# Clean build and VM artifacts
clean:
    rm -rf bin/ .vm/

