// Package ignition generates Butane YAML configs from InstallConfig.
// The generated YAML is Flatcar variant, spec 1.1.0, compiled to Ignition
// JSON via the coreos/butane Go library (not the CLI, which isn't on Flatcar).
//
// # Adding a new conditional section
//
// Historical state: a single multi-hundred-line raw string template held every
// section (storage, systemd, passwd). New features had to be hand-indented in
// the right place, and template errors only surfaced at render time (#88).
//
// Going forward: new features should use the [Builder] in builder.go.
// Each feature contributes its YAML fragment via [Builder.AddStorageFile],
// [Builder.AddStorageLink], or [Builder.AddSystemdUnit]; the Builder takes
// care of indentation and assembly. The Builder is end-to-end tested against
// [CompileToIgnition] so fragments that aren't valid Butane fail at unit-test
// time.
//
// [GenerateButane] still uses the legacy butaneTemplate for the existing
// surface (network, sysexts, NVIDIA, swap, Tailscale) — those will migrate
// in follow-up changes as the surrounding code is touched. New surface should
// be added via the Builder, not by extending butaneTemplate.
package ignition

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/projectbluefin/knuckle/internal/model"
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
			s = strings.ReplaceAll(s, "\n", `\n`)
			s = strings.ReplaceAll(s, "\r", `\r`)
			s = strings.ReplaceAll(s, "\t", `\t`)
			return s
		},
	}

	tmpl, err := template.New("butane").Funcs(funcMap).Parse(butaneTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	channel := cfg.Channel
	if channel == "" {
		channel = "stable"
	}

	rebootStrategy := cfg.UpdateStrategy.RebootStrategy
	if rebootStrategy == "" {
		rebootStrategy = "reboot"
	}

	// Enforce HTTPS for sysext download URLs before embedding in Ignition config.
	for _, s := range filterSelected(cfg.Sysexts) {
		if !strings.HasPrefix(s.URL, "https://") {
			return "", fmt.Errorf("sysext %q has non-HTTPS download URL %q", s.Name, s.URL)
		}
	}

	hasPassword := false
	for _, u := range cfg.Users {
		if u.PasswordHash != "" {
			hasPassword = true
			break
		}
	}

	swapSize := cfg.Swap.SizeMB
	if cfg.Swap.Enabled && swapSize <= 0 {
		swapSize = model.DefaultSwapSizeMB
	}

	tailscaleEnabled := hasTailscaleConfig(cfg.Tailscale)

	data := templateData{
		Hostname:            cfg.Hostname,
		Timezone:            cfg.Timezone,
		Users:               cfg.Users,
		SSHKeys:             cfg.SSHKeys,
		Network:             cfg.Network,
		Sysexts:             filterSelected(cfg.Sysexts),
		Channel:             channel,
		RebootStrategy:      rebootStrategy,
		HasPassword:         hasPassword,
		NvidiaDriverVersion: cfg.NvidiaDriverVersion,
		SwapEnabled:         cfg.Swap.Enabled,
		SwapSizeMB:          swapSize,
		Tailscale:           cfg.Tailscale,
		TailscaleEnabled:    tailscaleEnabled,
		TailscaleForwarding: tailscaleEnabled && (cfg.Tailscale.Mode == model.TailscaleModeExitNode || cfg.Tailscale.Mode == model.TailscaleModeSubnetRouter),
		TailscaleExtraArgs:  tailscaleExtraArgs(cfg.Tailscale),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

type templateData struct {
	Hostname            string
	Timezone            string
	Users               []model.UserConfig
	SSHKeys             []string
	Network             model.NetworkConfig
	Sysexts             []model.SysextEntry
	Channel             string
	RebootStrategy      string
	HasPassword         bool
	NvidiaDriverVersion string // e.g. "570-open"; empty = no NVIDIA kernel driver setup
	SwapEnabled         bool
	SwapSizeMB          int // effective swap size in MiB when SwapEnabled
	Tailscale           model.TailscaleConfig
	TailscaleEnabled    bool   // AuthKey is set, i.e. user filled the step
	TailscaleForwarding bool   // exit-node or subnet-router → need sysctl ip_forward=1
	TailscaleExtraArgs  string // value of TS_EXTRA_ARGS in tailscale.env
}

func hasTailscaleConfig(ts model.TailscaleConfig) bool {
	return strings.TrimSpace(ts.AuthKey) != ""
}

// tailscaleExtraArgs builds the TS_EXTRA_ARGS value for /etc/tailscale/tailscale.env
// based on the selected mode.
func tailscaleExtraArgs(ts model.TailscaleConfig) string {
	switch ts.Mode {
	case model.TailscaleModeExitNode:
		return "--advertise-exit-node"
	case model.TailscaleModeSubnetRouter:
		routes := strings.ReplaceAll(strings.TrimSpace(ts.Routes), " ", "")
		if routes == "" {
			return ""
		}
		return "--advertise-routes=" + routes
	default:
		return ""
	}
}

// GenerateFCOSButane produces a Butane YAML config string for Fedora CoreOS.
// The output uses variant: fcos, spec 1.5.0, with zincati for updates instead
// of update-engine. Flatcar-specific paths (/etc/flatcar/*) are excluded.
func (g *Generator) GenerateFCOSButane(cfg *model.InstallConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config cannot be nil")
	}

	for _, s := range filterSelected(cfg.Sysexts) {
		if !strings.HasPrefix(s.URL, "https://") {
			return "", fmt.Errorf("sysext %q has non-HTTPS download URL %q", s.Name, s.URL)
		}
	}

	b := NewBuilder(cfg)

	if err := addSharedFragments(b, cfg); err != nil {
		return "", err
	}

	fcosStrategy := cfg.UpdateStrategy.FCOSUpdateStrategy
	if fcosStrategy == "" {
		fcosStrategy = model.FCOSStrategyImmediate
	}

	var zincatiTmpl string
	if fcosStrategy == model.FCOSStrategyDisabled {
		zincatiTmpl = zincatiDisabledTemplate
	} else {
		zincatiTmpl = zincatiTemplate
	}
	zincatiTOML, err := renderTemplate("zincati", zincatiTmpl, struct{ FCOSUpdateStrategy string }{
		FCOSUpdateStrategy: fcosStrategy,
	})
	if err != nil {
		return "", fmt.Errorf("rendering zincati config: %w", err)
	}
	b.AddStorageFile(zincatiTOML)

	if len(filterSelected(cfg.Sysexts)) > 0 {
		b.AddSystemdUnit("- name: systemd-sysext.service\n  enabled: true")
	}
	b.AddSystemdUnit("- name: zincati.service\n  enabled: true")

	addSwapUnits(b, cfg)
	addTailscaleUnits(b, cfg)

	return b.BuildFCOS(), nil
}

//nolint:gochecknoglobals // var to allow test injection of error paths.
var zincatiTemplate = `- path: /etc/zincati/config.d/55-updates.toml
  mode: 0644
  overwrite: true
  contents:
    inline: |
      [updates]
      strategy = "{{.FCOSUpdateStrategy | yamlEscape}}"`

//nolint:gochecknoglobals // var to allow test injection of error paths.
var zincatiDisabledTemplate = `- path: /etc/zincati/config.d/55-updates.toml
  mode: 0644
  overwrite: true
  contents:
    inline: |
      [updates]
      enabled = false`

// Fragment templates used by addSharedFragments. Vars (not const) to allow
// test injection of error paths, following the same pattern as butaneTemplate.
//
//nolint:gochecknoglobals // var to allow test injection of error paths.
var (
	hostnameTemplate = `- path: /etc/hostname
  mode: 0644
  overwrite: true
  contents:
    inline: "{{.Hostname | yamlEscape}}"`

	sshHardeningTemplate = `- path: /etc/ssh/sshd_config.d/99-knuckle-hardening.conf
  mode: 0600
  overwrite: true
  contents:
    inline: |
      PasswordAuthentication {{if .HasPassword}}yes{{else}}no{{end}}
      PermitRootLogin no
      PubkeyAuthentication yes`

	staticNetTemplate = `- path: /etc/systemd/network/10-static.network
  mode: 0644
  contents:
    inline: |
      [Match]
      Name={{.Interface}}

      [Network]
      Address={{.Address}}
      Gateway={{.Gateway}}
{{- range .DNS}}
      DNS={{.}}
{{- end}}`

	timezoneTemplate = `- path: /etc/localtime
  target: "/usr/share/zoneinfo/{{.Timezone | yamlEscape}}"
  overwrite: true`
)

// addSharedFragments adds storage files, links, and passwd entries that are
// common to both Flatcar and FCOS variants.
func addSharedFragments(b *Builder, cfg *model.InstallConfig) error {
	hostnameFrag, err := renderTemplate("hostname", hostnameTemplate,
		struct{ Hostname string }{Hostname: cfg.Hostname})
	if err != nil {
		return fmt.Errorf("rendering hostname: %w", err)
	}
	b.AddStorageFile(hostnameFrag)

	hasPassword := false
	for _, u := range cfg.Users {
		if u.PasswordHash != "" {
			hasPassword = true
			break
		}
	}
	sshFrag, err := renderTemplate("ssh-hardening", sshHardeningTemplate,
		struct{ HasPassword bool }{HasPassword: hasPassword})
	if err != nil {
		return fmt.Errorf("rendering ssh hardening: %w", err)
	}
	b.AddStorageFile(sshFrag)

	if cfg.Network.Mode == model.NetworkStatic {
		netFrag, err := renderTemplate("static-net", staticNetTemplate, cfg.Network)
		if err != nil {
			return fmt.Errorf("rendering static network: %w", err)
		}
		b.AddStorageFile(netFrag)
	}

	for _, s := range filterSelected(cfg.Sysexts) {
		sysextFrag, err := renderTemplate("sysext-"+s.Name, sysextTemplate, s)
		if err != nil {
			return fmt.Errorf("rendering sysext %q: %w", s.Name, err)
		}
		b.AddStorageFile(sysextFrag)
	}

	if cfg.Swap.Enabled {
		b.AddStorageFile(`- path: /var/swapfile
  mode: 0600
  contents:
    source: "data:,"`)
	}

	tailscaleEnabled := hasTailscaleConfig(cfg.Tailscale)
	if tailscaleEnabled {
		tsFrag, err := renderTemplate("tailscale-env", tailscaleEnvTemplate, struct {
			Tailscale          model.TailscaleConfig
			TailscaleExtraArgs string
		}{
			Tailscale:          cfg.Tailscale,
			TailscaleExtraArgs: tailscaleExtraArgs(cfg.Tailscale),
		})
		if err != nil {
			return fmt.Errorf("rendering tailscale env: %w", err)
		}
		b.AddStorageFile(tsFrag)

		if cfg.Tailscale.Mode == model.TailscaleModeExitNode || cfg.Tailscale.Mode == model.TailscaleModeSubnetRouter {
			b.AddStorageFile(`- path: /etc/sysctl.d/99-tailscale.conf
  mode: 0644
  overwrite: true
  contents:
    inline: |
      net.ipv4.ip_forward = 1
      net.ipv6.conf.all.forwarding = 1`)
		}
	}

	if cfg.Timezone != "" {
		tzFrag, err := renderTemplate("timezone", timezoneTemplate,
			struct{ Timezone string }{Timezone: cfg.Timezone})
		if err != nil {
			return fmt.Errorf("rendering timezone: %w", err)
		}
		b.AddStorageLink(tzFrag)
	}

	passwdFrag, err := renderTemplate("passwd", passwdTemplate, struct {
		Users   []model.UserConfig
		SSHKeys []string
	}{Users: cfg.Users, SSHKeys: cfg.SSHKeys})
	if err != nil {
		return fmt.Errorf("rendering passwd: %w", err)
	}
	b.SetPasswdUsers(passwdFrag)

	return nil
}

//nolint:gochecknoglobals // var to allow test injection of error paths.
var sysextTemplate = `- path: /etc/extensions/{{.Name}}.raw
  contents:
    source: "{{.URL | yamlEscape}}"
{{- if .Sha256}}
    verification:
      hash: "sha256-{{.Sha256}}"
{{- end}}`

//nolint:gochecknoglobals // var to allow test injection of error paths.
var tailscaleEnvTemplate = `- path: /etc/tailscale/tailscale.env
  mode: 0600
  overwrite: true
  contents:
    inline: |
      TS_AUTHKEY={{.Tailscale.AuthKey | yamlEscape}}
      TS_AUTH_ONCE=true
{{- if .TailscaleExtraArgs}}
      TS_EXTRA_ARGS={{.TailscaleExtraArgs | yamlEscape}}
{{- end}}`

//nolint:gochecknoglobals // var to allow test injection of error paths.
var passwdTemplate = `{{- if .Users}}{{- range .Users}}- name: "{{.Username | yamlEscape}}"
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
{{end}}{{- else}}- name: "core"
{{- if .SSHKeys}}
  ssh_authorized_keys:
{{- range .SSHKeys}}
    - "{{. | yamlEscape}}"
{{- end}}
{{- end}}
{{end}}`

// addSwapUnits adds swap-related systemd units if swap is enabled.
func addSwapUnits(b *Builder, cfg *model.InstallConfig) {
	if !cfg.Swap.Enabled {
		return
	}
	swapSize := cfg.Swap.SizeMB
	if swapSize <= 0 {
		swapSize = model.DefaultSwapSizeMB
	}
	createFrag, _ := renderTemplate("swap-create", `- name: knuckle-create-swapfile.service
  enabled: true
  contents: |
    [Unit]
    Description=Create the /var/swapfile (knuckle, {{.SwapSizeMB}} MiB)
    Before=var-swapfile.swap
    ConditionPathExists=!/var/lib/knuckle/.swap-created

    [Service]
    Type=oneshot
    ExecStart=/usr/bin/fallocate -l {{.SwapSizeMB}}M /var/swapfile
    ExecStart=/usr/bin/chmod 0600 /var/swapfile
    ExecStart=/usr/sbin/mkswap /var/swapfile
    ExecStartPost=/bin/sh -c 'install -m 0644 -D /dev/null /var/lib/knuckle/.swap-created'
    RemainAfterExit=yes

    [Install]
    WantedBy=multi-user.target`, struct{ SwapSizeMB int }{SwapSizeMB: swapSize})
	b.AddSystemdUnit(createFrag)

	b.AddSystemdUnit(`- name: var-swapfile.swap
  enabled: true
  contents: |
    [Unit]
    Description=Activate /var/swapfile
    Requires=knuckle-create-swapfile.service
    After=knuckle-create-swapfile.service

    [Swap]
    What=/var/swapfile

    [Install]
    WantedBy=multi-user.target`)
}

// addTailscaleUnits adds Tailscale systemd units if Tailscale is configured.
func addTailscaleUnits(b *Builder, cfg *model.InstallConfig) {
	if !hasTailscaleConfig(cfg.Tailscale) {
		return
	}
	b.AddSystemdUnit("- name: tailscaled.service\n  enabled: true")
	tsFrag, _ := renderTemplate("tailscale-up", `- name: knuckle-tailscale-up.service
  enabled: true
  contents: |
    [Unit]
    Description=Bring up Tailscale with the auth key provisioned by knuckle
    Requires=tailscaled.service network-online.target
    After=tailscaled.service network-online.target
    ConditionPathExists=/etc/tailscale/tailscale.env
    ConditionPathExists=!/var/lib/tailscale/.knuckle-up-done

    [Service]
    Type=oneshot
    EnvironmentFile=/etc/tailscale/tailscale.env
    ExecStart=/usr/bin/tailscale up --auth-key=$${TS_AUTHKEY} $${TS_EXTRA_ARGS}
    ExecStartPost=/bin/sh -c 'install -m 0600 /dev/null /var/lib/tailscale/.knuckle-up-done'
    RemainAfterExit=yes

    [Install]
    WantedBy=multi-user.target`, nil)
	b.AddSystemdUnit(tsFrag)
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

//nolint:gochecknoglobals // var to allow test injection of error paths.
var butaneTemplate = `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      overwrite: true
      contents:
        inline: "{{.Hostname | yamlEscape}}"
    - path: /etc/flatcar/update.conf
      mode: 0644
      overwrite: true
      contents:
        inline: |
          REBOOT_STRATEGY={{.RebootStrategy}}
          GROUP={{.Channel}}
    - path: /etc/ssh/sshd_config.d/99-knuckle-hardening.conf
      mode: 0600
      overwrite: true
      contents:
        inline: |
          PasswordAuthentication {{if .HasPassword}}yes{{else}}no{{end}}
          PermitRootLogin no
          PubkeyAuthentication yes
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
        source: "{{.URL | yamlEscape}}"
{{- if .Sha256}}
        verification:
          hash: "sha256-{{.Sha256}}"
{{- end}}
{{- end}}
{{- if .NvidiaDriverVersion}}
    - path: /etc/flatcar/enabled-sysext.conf
      mode: 0644
      overwrite: true
      contents:
        inline: |
          nvidia-drivers-{{.NvidiaDriverVersion | yamlEscape}}
{{- end}}
{{- if .SwapEnabled}}
    - path: /var/swapfile
      mode: 0600
      contents:
        source: "data:,"
{{- end}}
{{- if .TailscaleEnabled}}
    - path: /etc/tailscale/tailscale.env
      mode: 0600
      overwrite: true
      contents:
        inline: |
          TS_AUTHKEY={{.Tailscale.AuthKey | yamlEscape}}
          TS_AUTH_ONCE=true
{{- if .TailscaleExtraArgs}}
          TS_EXTRA_ARGS={{.TailscaleExtraArgs | yamlEscape}}
{{- end}}
{{- if .TailscaleForwarding}}
    - path: /etc/sysctl.d/99-tailscale.conf
      mode: 0644
      overwrite: true
      contents:
        inline: |
          net.ipv4.ip_forward = 1
          net.ipv6.conf.all.forwarding = 1
{{- end}}
{{- end}}
{{- if .Timezone}}
  links:
    - path: /etc/localtime
      target: "/usr/share/zoneinfo/{{.Timezone | yamlEscape}}"
      overwrite: true
{{- end}}
systemd:
  units:
{{- if .Sysexts}}
    - name: systemd-sysext.service
      enabled: true
{{- end}}
    - name: update-engine.service
      enabled: true
{{- if .SwapEnabled}}
    - name: knuckle-create-swapfile.service
      enabled: true
      contents: |
        [Unit]
        Description=Create the /var/swapfile (knuckle, {{.SwapSizeMB}} MiB)
        Before=var-swapfile.swap
        ConditionPathExists=!/var/lib/knuckle/.swap-created

        [Service]
        Type=oneshot
        ExecStart=/usr/bin/fallocate -l {{.SwapSizeMB}}M /var/swapfile
        ExecStart=/usr/bin/chmod 0600 /var/swapfile
        ExecStart=/usr/sbin/mkswap /var/swapfile
        ExecStartPost=/bin/sh -c 'install -m 0644 -D /dev/null /var/lib/knuckle/.swap-created'
        RemainAfterExit=yes

        [Install]
        WantedBy=multi-user.target
    - name: var-swapfile.swap
      enabled: true
      contents: |
        [Unit]
        Description=Activate /var/swapfile
        Requires=knuckle-create-swapfile.service
        After=knuckle-create-swapfile.service

        [Swap]
        What=/var/swapfile

        [Install]
        WantedBy=multi-user.target
{{- end}}
{{- if .TailscaleEnabled}}
    - name: tailscaled.service
      enabled: true
    - name: knuckle-tailscale-up.service
      enabled: true
      contents: |
        [Unit]
        Description=Bring up Tailscale with the auth key provisioned by knuckle
        Requires=tailscaled.service network-online.target
        After=tailscaled.service network-online.target
        ConditionPathExists=/etc/tailscale/tailscale.env
        ConditionPathExists=!/var/lib/tailscale/.knuckle-up-done

        [Service]
        Type=oneshot
        EnvironmentFile=/etc/tailscale/tailscale.env
        ExecStart=/usr/bin/tailscale up --auth-key=$${TS_AUTHKEY} $${TS_EXTRA_ARGS}
        ExecStartPost=/bin/sh -c 'install -m 0600 /dev/null /var/lib/tailscale/.knuckle-up-done'
        RemainAfterExit=yes

        [Install]
        WantedBy=multi-user.target
{{- end}}
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
