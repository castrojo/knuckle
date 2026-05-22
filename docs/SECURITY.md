# Security Posture — knuckle

> Threat model, supply-chain claims (vs reality), and disclosure path. This
> file is owned by the principal-engineer review; update on every release.

## Scope

knuckle is a privileged TUI that drives `flatcar-install` on bare metal. It
touches three security-relevant surfaces:

1. **Disk** — selects and partitions the target device via `flatcar-install`.
2. **Network** — fetches sysext catalogs, Flatcar release metadata, and SSH
   keys from GitHub.
3. **Secrets** — assembles SSH public keys and bcrypt-hashed passwords into
   an Ignition config written to a temp file.

This document covers each.

## Threat Model

| Asset                         | Threat                                              | Mitigation                                                                       |
| ----------------------------- | --------------------------------------------------- | -------------------------------------------------------------------------------- |
| Target disk contents          | Wrong-disk wipe                                     | `/dev/disk/by-id` paths; boot disk auto-filtered in `internal/probe`              |
| Target disk contents          | TOCTOU between probe and `flatcar-install`          | `flatcar-install` re-probes; knuckle does not pre-format                         |
| Target disk contents          | Stale signatures / misplaced GPT backup header      | `wipefs --all --force` runs before install; `sfdisk --relocate gpt-bak-std` runs after imaging |
| Ignition file (SSH keys, pw)  | World-readable temp file                            | `os.CreateTemp` (O_EXCL) + `chmod 0600` + deferred unlink                        |
| Sysext download               | Tampered binary in transit                          | TLS via Go stdlib; SHA512 verified for Flatcar SBOM (display-only today)         |
| Sysext catalog                | Malicious release injected into `flatcar/sysext-bakery` | GitHub Releases API only; *no* signature verification — see Known Gaps          |
| GitHub SSH key fetch          | API impersonation                                   | TLS + hostname pinned to `github.com`                                            |
| Logs (`/tmp/knuckle.log`)     | Leak SSH keys / hashes                              | Logger writes structured `slog` events; secrets are not logged. Verify in review |
| Reboot                        | Unintended reboot in test                           | Double-key (`r` twice) confirmation in TUI; headless never reboots host            |

## Supply Chain

### What's verified today

- **Module pinning.** `go.mod` / `go.sum` are tidy-clean (`go mod tidy &&
  git diff --exit-code` runs in CI).
- **Vulnerability scan.** `govulncheck ./...` runs in `.github/workflows/ci.yml`
  on every PR. Build fails on advisories matching the call graph.
- **Static analysis.** CodeQL with the `security-and-quality` suite runs on
  every push, PR, and weekly schedule (`.github/workflows/security.yml`).
- **Dependency review.** GitHub's `dependency-review-action` blocks PRs that
  introduce dependencies with high-severity CVEs.
- **OSSF Scorecard.** Runs on push to main and weekly; results uploaded as
  SARIF to the code-scanning tab.
- **Flatcar SBOM.** `internal/bakery/channels.go` fetches the official
  `flatcar_production_image_sbom.json`, verifies SHA512 against the matching
  `.DIGESTS` file, and surfaces the result in the channel screen as
  `DigestVerified`.
- **Cosign keyless signing.** Every release binary (`knuckle`) and ISO
  (`knuckle-installer-stable.iso`) is signed with `cosign sign-blob --yes`
  using GitHub Actions OIDC (keyless). Signatures are recorded in the Sigstore
  Rekor transparency log. Bundles (`.bundle` files) are published alongside
  release artifacts.

### Verifying a release (users)

No GPG required. Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) then:

```sh
# Download the binary and its bundle from the GitHub release page, then:
cosign verify-blob \
  --bundle knuckle.bundle \
  --certificate-identity-regexp \
    "https://github.com/projectbluefin/knuckle/.github/workflows/release.yml@refs/tags/.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  knuckle
```

The same pattern applies to `knuckle-installer-stable.iso` / `knuckle-installer-stable.iso.bundle`.

### Known gaps (tracked, not fixed)

- **N-SEC1 MEDIUM — sysext download URLs are not length-checked.** `bakery.go`
  writes user-supplied sysext URLs into the Ignition config without a max-length
  guard. A very long URL is legal YAML but could produce an oversized Ignition
  file. Tracked MEDIUM.
- **N-SEC3 LOW — release server URLs are DNS-trusted.** `channels.go` contacts
  `{stable,beta,alpha,lts}.release.flatcar-linux.net` but only verifies TLS; no
  hostname pinning beyond the certificate chain. Acceptable for the threat model
  but worth noting.
- **Flatcar release GPG signatures are cryptographically verified.**
  `channels.go` downloads `.DIGESTS.asc` and `verify.go` validates the
  signature against the embedded Flatcar signing key
  (`internal/bakery/keys/flatcar-signing.asc`). If verification fails,
  `SignedDigest` is set to false and a warning is logged. This is separate
  from knuckle’s own release artifact signing (cosign keyless via Sigstore).
- **Sysext bakery downloads are unverified.** `internal/bakery/bakery.go`
  reads the GitHub Releases API but does not validate the `.sha256` or `.sig`
  alongside each `.raw` artifact. Tracked MEDIUM.
- **No fuzz testing on input parsers.** `internal/validate`,
  `internal/ignition` parsers are reachable from user input but not fuzzed.
  Tracked MEDIUM.
- **TOCTOU LOW in `WriteIgnitionFile`.** `os.CreateTemp` creates the file with
  correct mode on Linux (kernel enforces O_EXCL atomically), but a `chmod 0600`
  call follows the `CreateTemp`. The window is theoretical on any modern Linux
  with a private temp dir. Acknowledged, not fixed.

These gaps are listed because lying about them is worse than having them.
Don't claim verification in a PR description without checking the code.

## Secret Handling

The only secrets knuckle handles are:

- SSH **public** keys (not sensitive, but PII-adjacent).
- **bcrypt-hashed** user passwords — hashing happens client-side via
  `golang.org/x/crypto/bcrypt`. The plaintext never leaves the form field.

Both end up in an Ignition JSON file. Knuckle's invariants:

1. Ignition is written via `os.CreateTemp` (O_EXCL) → `chmod 0600` →
   `os.Remove` deferred. See `internal/install/install.go:WriteIgnitionFile`.
2. Ignition path is passed to `flatcar-install` by file, never by env or
   stdin (env is `proc/<pid>/environ`-readable; stdin can be inspected via
   `lsof`).
3. The structured logger (`slog`) is wired to a file handler, never stdout
   (Bubble Tea owns stdout). Log redaction is *not* automatic — reviewers
   must confirm new log lines never include secret material.

## Network Posture

- All HTTP clients use the Go stdlib (`net/http`) with a 10–30 s timeout.
  No insecure transports, no `InsecureSkipVerify`.
- Response sizes are capped (`io.LimitReader`, 5 MiB) in `channels.go` to
  bound memory on hostile servers.
- The set of contacted hosts is small and easy to audit:
  - `https://github.com` — SSH keys
  - `https://api.github.com` — sysext catalog
  - `https://{stable,beta,alpha,lts}.release.flatcar-linux.net` — releases

When adding a new endpoint, list it here. PRs that add a host without
updating this section should be blocked.

## Disclosure

Security issues should be reported privately:

- GitHub: open a [security advisory](https://github.com/projectbluefin/knuckle/security/advisories)
  on `projectbluefin/knuckle`.
- Do **not** open a public issue for a vulnerability.

We will acknowledge within 5 business days. Coordinated disclosure timeline
is 90 days from acknowledgement unless the issue is actively exploited, in
which case we will move faster.

## Review Cadence

- Every push triggers CodeQL + dependency-review (PR only) + Scorecard
  (main only) + `govulncheck`.
- Weekly Monday cron re-runs CodeQL + Scorecard against `main` so advisories
  filed after the last commit are still caught.
- Each release tag triggers a manual principal-engineer review using the
  checklist in `AGENTS.md` and the latest `docs/REVIEW-YYYY-MM-DD.md`.
