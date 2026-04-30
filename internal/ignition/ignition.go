// Package ignition generates Butane YAML configs from InstallConfig.
// The generated YAML is Flatcar variant, spec 1.1.0, and can be compiled
// to Ignition JSON via the butane CLI at install time.
package ignition

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/castrojo/knuckle/internal/model"
)

// Generator produces Butane YAML configs from InstallConfig.
type Generator struct{}

// NewGenerator returns a new Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateButane produces a Butane YAML config string from the given InstallConfig.
// The output is Flatcar variant, spec 1.1.0.
func (g *Generator) GenerateButane(cfg *model.InstallConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config cannot be nil")
	}

	funcMap := template.FuncMap{
		"isStatic": func(n model.NetworkConfig) bool {
			return n.Mode == model.NetworkStatic
		},
		"yamlEscape": func(s string) string {
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
			return s
		},
	}

	tmpl, err := template.New("butane").Funcs(funcMap).Parse(butaneTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	rebootStrategy := cfg.UpdateStrategy.RebootStrategy
	if rebootStrategy == "" {
		rebootStrategy = "reboot"
	}

	hasPassword := false
	for _, u := range cfg.Users {
		if u.PasswordHash != "" {
			hasPassword = true
			break
		}
	}

	data := templateData{
		Hostname:       cfg.Hostname,
		Timezone:       cfg.Timezone,
		Users:          cfg.Users,
		SSHKeys:        cfg.SSHKeys,
		Network:        cfg.Network,
		Sysexts:        filterSelected(cfg.Sysexts),
		Channel:        cfg.Channel,
		RebootStrategy: rebootStrategy,
		HasPassword:    hasPassword,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

type templateData struct {
	Hostname       string
	Timezone       string
	Users          []model.UserConfig
	SSHKeys        []string
	Network        model.NetworkConfig
	Sysexts        []model.SysextEntry
	Channel        string
	RebootStrategy string
	HasPassword    bool
}

func filterSelected(sysexts []model.SysextEntry) []model.SysextEntry {
	var selected []model.SysextEntry
	for _, s := range sysexts {
		if s.Selected {
			selected = append(selected, s)
		}
	}
	return selected
}

const butaneTemplate = `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      contents:
        inline: "{{.Hostname | yamlEscape}}"
    - path: /etc/flatcar/update.conf
      mode: 0644
      contents:
        inline: |
          REBOOT_STRATEGY={{.RebootStrategy}}
          GROUP={{.Channel}}
    - path: /etc/ssh/sshd_config.d/99-knuckle-hardening.conf
      mode: 0600
      contents:
        inline: |
          PasswordAuthentication {{if .HasPassword}}yes{{else}}no{{end}}
          PermitRootLogin no
          PubkeyAuthentication yes
{{- if .Timezone}}
  links:
    - path: /etc/localtime
      target: "/usr/share/zoneinfo/{{.Timezone}}"
      overwrite: true
{{- end}}
{{- if isStatic .Network}}
    - path: /etc/systemd/network/10-static.network
      mode: 0644
      contents:
        inline: |
          [Match]
          Name={{.Network.Interface}}

          [Network]
          Address={{.Network.Address}}
          Gateway={{.Network.Gateway}}
{{- range .Network.DNS}}
          DNS={{.}}
{{- end}}
{{- end}}
{{- range .Sysexts}}
    - path: /etc/extensions/{{.Name}}.raw
      contents:
        source: "{{.URL}}"
{{- end}}
systemd:
  units:
{{- if .Sysexts}}
    - name: systemd-sysext.service
      enabled: true
{{- end}}
    - name: update-engine.service
      enabled: true
passwd:
  users:
{{- if .Users}}
{{- range .Users}}
    - name: "{{.Username | yamlEscape}}"
{{- if .Groups}}
      groups:
{{- range .Groups}}
        - "{{. | yamlEscape}}"
{{- end}}
{{- end}}
{{- if .SSHKeys}}
      ssh_authorized_keys:
{{- range .SSHKeys}}
        - "{{. | yamlEscape}}"
{{- end}}
{{- end}}
{{- if .PasswordHash}}
      password_hash: "{{.PasswordHash | yamlEscape}}"
{{- end}}
{{- end}}
{{- else}}
    - name: "core"
{{- if .SSHKeys}}
      ssh_authorized_keys:
{{- range .SSHKeys}}
        - "{{. | yamlEscape}}"
{{- end}}
{{- end}}
{{- end}}
`
