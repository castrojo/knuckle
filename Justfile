# Knuckle — Flatcar Container Linux TUI Installer
# https://github.com/castrojo/knuckle

QEMU := if path_exists("/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64") == "true" { "/home/linuxbrew/.linuxbrew/bin/qemu-system-x86_64" } else { "/usr/bin/qemu-system-x86_64" }
OVMF := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
ISO := ".vm/flatcar_production_iso_image.iso"
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

# Download Flatcar stable ISO for VM testing
download-iso:
    mkdir -p .vm
    @if [ ! -f {{ISO}} ]; then \
        echo "Downloading Flatcar stable ISO..."; \
        curl -L -o {{ISO}} "https://stable.release.flatcar-linux.net/amd64-usr/current/flatcar_production_iso_image.iso"; \
    else \
        echo "ISO already exists: {{ISO}}"; \
    fi

# Create a test disk image for VM
create-disk:
    mkdir -p .vm
    qemu-img create -f qcow2 {{DISK}} 20G

# Boot Flatcar VM with knuckle binary available via virtio-9p
vm: build-linux download-iso create-disk
    {{QEMU}} \
        -m 2048 \
        -smp 2 \
        -enable-kvm \
        -drive if=pflash,format=raw,readonly=on,file={{OVMF}} \
        -cdrom {{ISO}} \
        -drive file={{DISK}},format=qcow2,if=virtio \
        -netdev user,id=net0,hostfwd=tcp:127.0.0.1:2222-:22 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path=bin,mount_tag=knuckle,security_model=mapped-xattr,id=knuckle \
        -nographic

# Boot Flatcar VM with graphical display
vm-gui: build-linux download-iso create-disk
    {{QEMU}} \
        -m 2048 \
        -smp 2 \
        -enable-kvm \
        -drive if=pflash,format=raw,readonly=on,file={{OVMF}} \
        -cdrom {{ISO}} \
        -drive file={{DISK}},format=qcow2,if=virtio \
        -netdev user,id=net0,hostfwd=tcp:127.0.0.1:2222-:22 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path=bin,mount_tag=knuckle,security_model=mapped-xattr,id=knuckle

# SSH into running VM (default Flatcar user: core)
ssh:
    ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 core@127.0.0.1

# Clean build and VM artifacts
clean:
    rm -rf bin/ .vm/

