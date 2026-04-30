// Package iso generates Ignition configs and helpers for building
// self-contained Flatcar installer disk images with knuckle embedded.
package iso

import "encoding/json"

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

// knuckleServiceUnit is the systemd unit that launches knuckle on tty1.
const knuckleServiceUnit = `[Unit]
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

// GenerateInstallerIgnition creates Ignition JSON for the installer image.
// The knuckle binary must be placed at /opt/knuckle on the filesystem
// (via virt-customize or equivalent) before booting with this config.
//
// If sshPubKey is non-empty, it is added to the "core" user for debug access.
func GenerateInstallerIgnition(sshPubKey string) ([]byte, error) {
	enabled := true

	cfg := ignitionConfig{
		Ignition: ignitionMeta{Version: "3.3.0"},
		Systemd: systemdCfg{
			Units: []unit{
				{Name: "sshd.service", Enabled: &enabled},
				{
					Name:     "knuckle-installer.service",
					Enabled:  &enabled,
					Contents: knuckleServiceUnit,
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
