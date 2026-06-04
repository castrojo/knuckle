// Package iso generates Ignition configs and helpers for building
// self-contained installer disk images with knuckle embedded.
package iso

import (
	"encoding/json"
	"fmt"
)

const (
	// OSFlatcar targets Flatcar Container Linux live images.
	OSFlatcar = "flatcar"
	// OSFCOS targets Fedora CoreOS live images.
	OSFCOS = "fcos"
)

// ignitionConfig is the minimal Ignition 3.3.0 schema needed for the installer.
type ignitionConfig struct {
	Ignition ignitionMeta `json:"ignition"`
	Systemd  systemdCfg   `json:"systemd"`
	Passwd   *passwdCfg   `json:"passwd,omitempty"`
}

type ignitionMeta struct {
	Version string `json:"version"`
}

type systemdCfg struct {
	Units []unit `json:"units"`
}

type unit struct {
	Name     string `json:"name"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Contents string `json:"contents,omitempty"`
}

type passwdCfg struct {
	Users []user `json:"users,omitempty"`
}

type user struct {
	Name              string   `json:"name"`
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`
}

// knuckleFlatcarServiceUnit is the systemd unit for Flatcar live images.
const knuckleFlatcarServiceUnit = `[Unit]
Description=Knuckle Flatcar Installer
After=multi-user.target
ConditionPathExists=/opt/knuckle

[Service]
Type=idle
ExecStart=/opt/knuckle
StandardInput=tty
StandardOutput=tty
TTYPath=/dev/tty1
TTYReset=yes
TTYVHangup=yes
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target`

// knuckleFCOSServiceUnit is the systemd unit for FCOS live images.
// FCOS runs getty@tty1.service with autologin for the core user;
// Conflicts= and Before= ensure knuckle wins tty1.
const knuckleFCOSServiceUnit = `[Unit]
Description=Knuckle FCOS Installer
After=multi-user.target
Conflicts=getty@tty1.service
Before=getty@tty1.service
ConditionPathExists=/opt/knuckle

[Service]
Type=idle
ExecStart=/opt/knuckle
StandardInput=tty
StandardOutput=tty
TTYPath=/dev/tty1
TTYReset=yes
TTYVHangup=yes
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target`

// serviceUnitForOS returns the correct knuckle systemd unit for the target OS.
func serviceUnitForOS(targetOS string) (string, error) {
	switch targetOS {
	case OSFlatcar, "":
		return knuckleFlatcarServiceUnit, nil
	case OSFCOS:
		return knuckleFCOSServiceUnit, nil
	default:
		return "", fmt.Errorf("unsupported target OS: %q", targetOS)
	}
}

// GenerateInstallerIgnition creates Ignition JSON for the installer image.
// The targetOS parameter selects OS-specific systemd unit directives
// (use OSFlatcar or OSFCOS; empty string defaults to Flatcar).
// The knuckle binary must be placed at /opt/knuckle on the filesystem
// before booting with this config.
//
// If sshPubKey is non-empty, it is added to the "core" user for debug access.
func GenerateInstallerIgnition(targetOS, sshPubKey string) ([]byte, error) {
	unitContents, err := serviceUnitForOS(targetOS)
	if err != nil {
		return nil, err
	}

	enabled := true

	cfg := ignitionConfig{
		Ignition: ignitionMeta{Version: "3.3.0"},
		Systemd: systemdCfg{
			Units: []unit{
				{Name: "sshd.service", Enabled: &enabled},
				{
					Name:     "knuckle-installer.service",
					Enabled:  &enabled,
					Contents: unitContents,
				},
			},
		},
	}

	if sshPubKey != "" {
		cfg.Passwd = &passwdCfg{
			Users: []user{{Name: "core", SSHAuthorizedKeys: []string{sshPubKey}}},
		}
	}

	return json.MarshalIndent(cfg, "", "  ")
}
