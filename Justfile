# Knuckle — Flatcar Container Linux TUI Installer
# https://github.com/castrojo/knuckle

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

# Cross-compile for linux/amd64
build-linux:
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/knuckle-linux-amd64 ./cmd/knuckle

# Run the TUI
run:
    go run ./cmd/knuckle

# Tidy dependencies
tidy:
    go mod tidy

# Run govulncheck
vuln:
    govulncheck ./...

# Full CI pipeline (tidy + lint + test + build)
ci: tidy lint test-race build
