# Release Checklist — knuckle

Run this before tagging any release. Every item must be green.

```bash
# One command to verify everything
just tools && just ci
```

## Gates

- [ ] `go mod tidy && git diff --exit-code go.mod go.sum` — module graph clean
- [ ] `gofmt -l .` empty
- [ ] `go vet ./...` clean
- [ ] `.tools/golangci-lint run ./...` clean
- [ ] `go tool govulncheck ./...` — `No vulnerabilities found.`
- [ ] `go test -race ./...` — all packages green
- [ ] `just cover-check` — all packages above gate thresholds
- [ ] `just headless-test` — config generation e2e passes
- [ ] `just vm-e2e` — all 4 passes green (DHCP · static · sysext · NVIDIA)
- [ ] `just build` — binary compiles
- [ ] `git status` clean — no untracked files
- [ ] `grep -rn 'exec\.Command' --include='*.go' --exclude-dir=internal/runner .` → zero results
- [ ] All claims in `README.md` still true
- [ ] `docs/internal/REVIEW-*.md` reconciled — every blocker fixed or deferred with issue

## VM Verification (required)

```bash
just vm       # manual TUI walkthrough — confirm install + SSH on installed system
just vm-e2e   # automated 4-pass — must exit 0
```

## Tag and Push

```bash
git tag v0.X.Y
git push origin v0.X.Y   # triggers release.yml: build → sign → publish
```

The release workflow (`release.yml`) builds amd64 + arm64 binaries, installer ISOs,
cosign keyless signatures, and publishes a GitHub Release. See `docs/GHOST-LAB.md`
for validating ARM64 artifacts before tagging.

## Blockers History

B1 (GPG) ✓ · B2 (reboot runner) ✓ · B3 (headless disk path) ✓ · B4 (SSH keys → Ignition) ✓

No open blockers for v1.0.
