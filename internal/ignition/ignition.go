// Package ignition generates Butane YAML configs from InstallConfig.
// The generated YAML is Flatcar variant, spec 1.1.0, and can be compiled
// to Ignition JSON via the butane CLI at install time.
package ignition

import (
	"bytes"
	"fmt"
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
	}

	tmpl, err := template.New("butane").Funcs(funcMap).Parse(butaneTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	data := templateData{
		Hostname: cfg.Hostname,
		Users:    cfg.Users,
		SSHKeys:  cfg.SSHKeys,
		Network:  cfg.Network,
		Sysexts:  filterSelected(cfg.Sysexts),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

type templateData struct {
	Hostname string
	Users    []model.UserConfig
	SSHKeys  []string
	Network  model.NetworkConfig
	Sysexts  []model.SysextEntry
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
        inline: "{{.Hostname}}"
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
{{- if .Sysexts}}
  links:
{{- range .Sysexts}}
    - path: /etc/extensions/{{.Name}}.raw
      target: /opt/extensions/{{.Name}}/{{.Name}}-{{.Version}}.raw
{{- end}}
{{- end}}
passwd:
  users:
{{- if .Users}}
{{- range .Users}}
    - name: "{{.Username}}"
{{- if .Groups}}
      groups:
{{- range .Groups}}
        - "{{.}}"
{{- end}}
{{- end}}
{{- if .SSHKeys}}
      ssh_authorized_keys:
{{- range .SSHKeys}}
        - "{{.}}"
{{- end}}
{{- end}}
{{- if .PasswordHash}}
      password_hash: "{{.PasswordHash}}"
{{- end}}
{{- end}}
{{- else}}
    - name: "core"
{{- if .SSHKeys}}
      ssh_authorized_keys:
{{- range .SSHKeys}}
        - "{{.}}"
{{- end}}
{{- end}}
{{- end}}
`
