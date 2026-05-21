# Changelog

All notable changes to knuckle are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Each
release also has a GitHub Release with auto-generated PR-level notes — this
file is the curated, human-readable history.

## [Unreleased]

### Added
- Flatcar Discord community links on the welcome and install-done screens
  (#117).
- `.github/release.yml` to group auto-generated release notes by label
  (Features, Bug fixes, Security, etc.) (#116).

## [0.6.2] - 2026-05-19

### Fixed
- `Shift+Tab` now goes back a wizard step instead of incorrectly moving the
  cursor up.

## [0.6.1] - 2026-05-18

### Fixed
- Use `/dev/console` in the installer systemd unit so the TUI renders correctly
  when booting over serial.

### Added
- Tests for GitHub SSH key-fetch error handling and no-auth guard on
  `StepUser`.

## [0.6.0] - 2026-05-17

### Fixed
- Password-only auth in headless mode now propagates to `InstallConfig`.

## [0.5.1] - 2026-05-16

### Added
- CNCF supply-chain stack: Syft SBOMs, cosign keyless signing, ORAS push to
  GHCR, SLSA build provenance attestations.

## [0.5.0] - 2026-05-15

### Fixed
- External Ignition URL path bypasses auth/channel consistency checks.

## [0.4.0] - 2026-05-14

### Fixed
- CI: use the correct arm64 runner label (`ubuntu-24.04-arm`).

## [0.3.0] - 2026-05-13

### Fixed
- `justfile`: escape Go template braces in the `vm-e2e` docker check.

## [0.2.1] - 2026-05-12

### Fixed
- ISO: inject `knuckle` into `usr.squashfs` for reliable live boot.

## [0.2.0] - 2026-05-11

### Security
- Real Flatcar GPG verification via ProtonMail/go-crypto (security baseline
  B1).

## [0.1.0] - 2026-05-10

### Added
- First working ISO build with GRUB and `boot-iso` justfile recipes.

[Unreleased]: https://github.com/projectbluefin/knuckle/compare/v0.6.2...HEAD
[0.6.2]: https://github.com/projectbluefin/knuckle/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/projectbluefin/knuckle/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/projectbluefin/knuckle/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/projectbluefin/knuckle/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/projectbluefin/knuckle/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/projectbluefin/knuckle/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/projectbluefin/knuckle/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/projectbluefin/knuckle/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/projectbluefin/knuckle/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/projectbluefin/knuckle/releases/tag/v0.1.0
