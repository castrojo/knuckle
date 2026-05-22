package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
)

// buildNetworkForm creates the huh form for the Network step.
func (m *Model) buildNetworkForm() *huh.Form {
	// Build interface options from detected interfaces
	ifaceOptions := []huh.Option[string]{
		huh.NewOption("Auto (DHCP on all interfaces)", ""),
	}
	for _, iface := range m.Wizard.State.Interfaces {
		label := fmt.Sprintf("%s — %s (%s)", iface.Name, iface.MAC, iface.State)
		ifaceOptions = append(ifaceOptions, huh.NewOption(label, iface.Name))
	}

	modeOptions := []huh.Option[string]{
		huh.NewOption("DHCP — automatic configuration (recommended)", "dhcp"),
		huh.NewOption("Static — manual IP configuration", "static"),
	}

	fields := []huh.Field{
		huh.NewNote().
			Title("Network Configuration").
			Description("How should this machine connect to the network?"),
		huh.NewSelect[string]().
			Title("Network Mode").
			Options(modeOptions...).
			Value(&m.networkModeInput),
	}

	// Only show static config fields if static mode is likely
	// (huh doesn't support conditional fields, so we always show them
	// but the Note explains they're only used for static)
	staticFields := []huh.Field{
		huh.NewSelect[string]().
			Title("Interface").
			Description("Which network interface to configure").
			Options(ifaceOptions...).
			Value(&m.Wizard.State.Config.Network.Interface),
		huh.NewInput().
			Title("IP Address").
			Description("With subnet mask, e.g. 192.168.1.100/24").
			Placeholder("192.168.1.100/24").
			Value(&m.Wizard.State.Config.Network.Address).
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				return validate.CIDR(s)
			}),
		huh.NewInput().
			Title("Gateway").
			Placeholder("192.168.1.1").
			Value(&m.Wizard.State.Config.Network.Gateway).
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				return validate.IPAddress(s)
			}),
		huh.NewInput().
			Title("DNS Servers").
			Description("Comma-separated, e.g. 1.1.1.1,8.8.8.8").
			Placeholder("1.1.1.1").
			Value(&m.dnsInput),
	}

	return huh.NewForm(
		huh.NewGroup(fields...),
		huh.NewGroup(staticFields...).Title("Static IP Configuration").
			Description("Only needed if you chose Static mode above"),
	).WithTheme(huh.ThemeDracula()).WithShowHelp(true).WithWidth(80)
}

// buildUserForm creates the huh form for the User step.
// Split into two groups so it feels like a wizard progression.
func (m *Model) buildUserForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("System Identity").
				Description("Configure the hostname and primary user account."),
			huh.NewInput().
				Title("Hostname").
				Placeholder("flatcar-node01").
				Value(&m.Wizard.State.Config.Hostname).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					return validate.Hostname(s)
				}),
			huh.NewInput().
				Title("Timezone").
				Placeholder("UTC").
				Description("e.g. America/New_York, Europe/Berlin").
				Value(&m.Wizard.State.Config.Timezone).
				Validate(func(s string) error {
					return validate.Timezone(s)
				}),
			huh.NewInput().
				Title("Username").
				Description("Primary user account").
				Value(&m.usernameInput).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					return validate.Username(s)
				}),
			huh.NewInput().
				Title("Password").
				Description("Optional — leave blank for key-only auth").
				EchoMode(huh.EchoModePassword).
				Value(&m.passwordInput),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("Authentication").
				Description("Set up SSH access. Your local SSH keys (~/.ssh/*.pub) are\nincluded automatically. Add a GitHub username to fetch additional\nkeys, or paste one directly."),
			huh.NewInput().
				Title("GitHub Username").
				Description("Fetches your SSH public keys automatically").
				Placeholder("username or @username").
				Value(&m.githubUserInput),
			huh.NewInput().
				Title("SSH Public Key").
				Description("Or paste key directly (separate multiple with ;)").
				Value(&m.sshKeyInput),
			huh.NewNote().
				Title("").
				Description(m.localKeysSummary()),
		),
	).WithTheme(huh.ThemeDracula()).WithShowHelp(true).WithWidth(80)
}

// localKeysSummary returns a description showing detected local SSH keys.
func (m *Model) localKeysSummary() string {
	keys := detectLocalSSHKeys()
	if len(keys) == 0 {
		return "⚠ No local SSH keys found in ~/.ssh/"
	}
	return fmt.Sprintf("🔑 %d local key(s) from ~/.ssh/ will be included automatically", len(keys))
}

// buildTailscaleForm creates the huh form for the Tailscale step.
// Shown only when the tailscale sysext is selected.
func (m *Model) buildTailscaleForm() *huh.Form {
	modeOptions := []huh.Option[string]{
		huh.NewOption("Just connect — plain client", model.TailscaleModeConnect),
		huh.NewOption("Exit node — advertise as exit node", model.TailscaleModeExitNode),
		huh.NewOption("Subnet router — advertise routes", model.TailscaleModeSubnetRouter),
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Tailscale").
				Description("Optional. Paste a tailnet auth key to provision Tailscale at first boot.\nLeave the auth key blank to skip — tailscale binaries will still be installed."),
			huh.NewInput().
				Title("Auth Key").
				Description("From https://login.tailscale.com/admin/settings/keys — preauth is recommended").
				Placeholder("tskey-auth-...").
				EchoMode(huh.EchoModePassword).
				Value(&m.tailscaleAuthKeyIn).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					return validate.TailscaleAuthKey(s)
				}),
			huh.NewSelect[string]().
				Title("Mode").
				Options(modeOptions...).
				Value(&m.tailscaleModeIn),
			huh.NewInput().
				Title("Advertised routes").
				Description("Only used in Subnet router mode. Comma-separated CIDRs (e.g. 10.0.0.0/24,192.168.1.0/24)").
				Placeholder("10.0.0.0/24").
				Value(&m.tailscaleRoutesIn),
		),
	).WithTheme(huh.ThemeDracula()).WithShowHelp(true).WithWidth(80)
}

// buildReviewForm creates the huh confirm for the Review step.
func (m *Model) buildReviewForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("⚠️  DESTRUCTIVE OPERATION — Install Flatcar to disk?").
				Description(m.reviewSummary()).
				Affirmative("Yes, install").
				Negative("Go back").
				Value(&m.Wizard.State.Confirmed),
		),
	).WithTheme(huh.ThemeDracula()).WithShowHelp(true).WithWidth(80)
}

func (m *Model) reviewSummary() string {
	cfg := &m.Wizard.State.Config
	var b strings.Builder
	fmt.Fprintf(&b, "Channel: %s", cfg.Channel)
	if cfg.Version != "" {
		fmt.Fprintf(&b, " (v%s)", cfg.Version)
	}
	fmt.Fprintf(&b, "\nDisk: %s", cfg.Disk.DevPath)
	if cfg.Disk.Model != "" {
		fmt.Fprintf(&b, " (%s, %s)", cfg.Disk.Model, cfg.Disk.SizeHuman)
	}
	fmt.Fprintf(&b, "\nNetwork: %s", cfg.Network.Mode)
	if cfg.Network.Mode == model.NetworkStatic {
		fmt.Fprintf(&b, " — %s via %s", cfg.Network.Address, cfg.Network.Gateway)
	}
	fmt.Fprintf(&b, "\nHostname: %s", cfg.Hostname)
	if len(cfg.Users) > 0 {
		fmt.Fprintf(&b, "\nUser: %s", cfg.Users[0].Username)
	}
	if len(cfg.SSHKeys) > 0 {
		fmt.Fprintf(&b, "\nSSH Keys: %d key(s)", len(cfg.SSHKeys))
	}
	if len(cfg.Sysexts) > 0 {
		names := make([]string, len(cfg.Sysexts))
		for i, s := range cfg.Sysexts {
			names[i] = s.Name
		}
		fmt.Fprintf(&b, "\nSysexts: %s", strings.Join(names, ", "))
	}
	if cfg.Swap.Enabled {
		size := cfg.Swap.SizeMB
		if size <= 0 {
			size = model.DefaultSwapSizeMB
		}
		fmt.Fprintf(&b, "\nSwap: %d MiB (/var/swapfile)", size)
	} else {
		b.WriteString("\nSwap: disabled")
	}
	if cfg.Tailscale.AuthKey != "" {
		mode := cfg.Tailscale.Mode
		if mode == "" {
			mode = model.TailscaleModeConnect
		}
		fmt.Fprintf(&b, "\nTailscale: auth key set, mode=%s", mode)
		if mode == model.TailscaleModeSubnetRouter && cfg.Tailscale.Routes != "" {
			fmt.Fprintf(&b, " routes=%s", cfg.Tailscale.Routes)
		}
	}
	return b.String()
}

// buildBreadcrumb kept for form_logic.go compatibility.
func (m *Model) buildBreadcrumb() string {
	return m.renderZenChrome()
}

// renderSystemChecks absorbed into zen chrome — returns empty.
func (m *Model) renderSystemChecks() string {
	return ""
}

// renderZenChrome creates the ANSI-art inspired header.
// Aesthetic: clean framed letterform, cool blue palette, scene-era vibes.
// Info shown via color hierarchy — version numbers always visible.
func (m *Model) renderZenChrome() string {
	var b strings.Builder

	// Color palette
	logoHi := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	logoLo := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	dimColor := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	infoColor := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	accentColor := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	okDot := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnDot := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	failDot := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	presentsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	sloganStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Italic(true)

	// Pretext
	b.WriteString(presentsStyle.Render("  Project Bluefin presents..."))
	b.WriteString("\n\n")

	// Logo: spaced letterform in double-line frame
	b.WriteString(logoLo.Render("\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557"))
	b.WriteString("\n")
	b.WriteString(logoLo.Render("\u2551") + "     " + logoHi.Render("K N U C K L E") + "                                        " + logoLo.Render("\u2551"))
	b.WriteString("\n")
	b.WriteString(logoLo.Render("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d"))
	b.WriteString("\n")

	// Subtitle + slogan
	b.WriteString("  ")
	b.WriteString(accentColor.Render("homelab ignition configurator"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(sloganStyle.Render("The real thing, right from the CNCF. Legends will rise."))
	b.WriteString("\n\n")

	// Info line: version + system dots (skip on Welcome — cards show it)
	cfg := &m.Wizard.State.Config
	if m.Wizard.State.CurrentStep != model.StepWelcome {

		// Channel as label, versions as tight key:value with │ separators
		var verInfo string
		if len(m.Wizard.State.Channels) > 0 {
			for _, ch := range m.Wizard.State.Channels {
				if ch.Channel == cfg.Channel {
					verInfo = accentColor.Render(ch.Channel) +
						dimColor.Render(" \u2502 ") +
						infoColor.Render("v"+ch.Version) +
						dimColor.Render(" \u2502 ") +
						infoColor.Render("linux "+ch.Kernel) +
						dimColor.Render(" \u2502 ") +
						infoColor.Render("systemd "+ch.Systemd)
					break
				}
			}
		}
		if verInfo == "" {
			verInfo = accentColor.Render(cfg.Channel)
		}

		b.WriteString("  ")
		b.WriteString(verInfo)

		if len(m.Wizard.State.SystemChecks) > 0 {
			b.WriteString(dimColor.Render("  \u2502  "))
			for i, check := range m.Wizard.State.SystemChecks {
				switch check.Status {
				case "ok":
					b.WriteString(okDot.Render("\u25cf"))
				case "warn":
					b.WriteString(warnDot.Render("\u25cf"))
				default:
					b.WriteString(failDot.Render("\u25cf"))
				}
				if i < len(m.Wizard.State.SystemChecks)-1 {
					b.WriteString(" ")
				}
			}
		}
		b.WriteString("\n")
	} // end if not Welcome

	// Step progress: thin line
	steps := 8
	current := int(m.Wizard.State.CurrentStep)
	b.WriteString("  ")
	for i := 0; i < steps; i++ {
		if i < current {
			b.WriteString(accentColor.Render("\u2501\u2501"))
		} else if i == current {
			b.WriteString(logoHi.Render("\u2501\u2501"))
		} else {
			b.WriteString(dimColor.Render("\u2500\u2500"))
		}
		if i < steps-1 {
			b.WriteString(dimColor.Render("\u00b7"))
		}
	}
	b.WriteString("\n\n")

	return b.String()
}

// channelList returns the ordered list of channel keys for the card selector.
func (m *Model) channelList() []string {
	return []string{"stable", "lts", "beta", "alpha"}
}

// channelCardCount returns how many channel cards to display.
func (m *Model) channelCardCount() int {
	return len(m.channelList())
}

// channelMeta holds display info for each channel card.
type channelMeta struct {
	name    string
	version string
	kernel  string
	systemd string
	docker  string
	desc    string
}

// getChannelMeta builds display metadata for each channel.
func (m *Model) getChannelMeta() []channelMeta {
	channels := m.channelList()
	metas := make([]channelMeta, len(channels))

	// Default descriptions
	descs := map[string]string{
		"stable": "Tested for production. Default for most deployments.",
		"lts":    "Long-term support. Extended maintenance window.",
		"beta":   "Next stable candidate. Test before production.",
		"alpha":  "Bleeding edge. New kernel, systemd, core packages.",
	}

	for i, ch := range channels {
		metas[i] = channelMeta{
			name: ch,
			desc: descs[ch],
		}
		// Fill in version info from fetched channel data
		for _, info := range m.Wizard.State.Channels {
			if info.Channel == ch {
				metas[i].version = info.Version
				metas[i].kernel = info.Kernel
				metas[i].systemd = info.Systemd
				metas[i].docker = info.Docker
				break
			}
		}
	}
	return metas
}

// viewChannelCards renders Flatcar-website-style channel selector cards.
func (m *Model) viewChannelCards() string {
	var b strings.Builder

	// Styles
	selectedBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("51")).
		Padding(0, 1).
		Width(60)
	normalBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(60)
	nameSelected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))
	nameNormal := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)

	b.WriteString("  Select a release channel:\n\n")

	metas := m.getChannelMeta()
	for i, meta := range metas {
		selected := i == m.cursor

		// Build card content
		var card strings.Builder

		// Line 1: cursor + name + version (right-aligned)
		cursor := "  "
		nameStyle := nameNormal
		if selected {
			cursor = cursorStyle.Render("▸ ")
			nameStyle = nameSelected
		}

		var displayName string
		if meta.name == "lts" {
			displayName = "LTS"
		} else {
			displayName = strings.ToUpper(meta.name[:1]) + meta.name[1:]
		}
		name := nameStyle.Render(displayName)
		ver := ""
		if meta.version != "" {
			ver = versionStyle.Render("v" + meta.version)
		}
		// Pad between name and version
		padding := 60 - 4 - len(meta.name) - len("v"+meta.version)
		if padding < 1 {
			padding = 1
		}
		card.WriteString(cursor + name + strings.Repeat(" ", padding) + ver)
		card.WriteString("\n")

		// Line 2: description
		card.WriteString("  " + descStyle.Render(meta.desc))

		// Line 3: component versions (if available)
		if meta.kernel != "" || meta.systemd != "" || meta.docker != "" {
			card.WriteString("\n")
			parts := []string{}
			if meta.kernel != "" {
				parts = append(parts, "linux "+meta.kernel)
			}
			if meta.systemd != "" {
				parts = append(parts, "systemd "+meta.systemd)
			}
			if meta.docker != "" {
				parts = append(parts, "docker "+meta.docker)
			}
			card.WriteString("  " + detailStyle.Render(strings.Join(parts, " · ")))
		}

		// Render with border
		if selected {
			b.WriteString(selectedBorder.Render(card.String()))
		} else {
			b.WriteString(normalBorder.Render(card.String()))
		}
		b.WriteString("\n")
	}

	// Show advanced hint + community link
	b.WriteString("\n")
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	link := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	b.WriteString(dim.Render("  Ctrl+A advanced options · ↑↓/jk select · enter continue"))
	b.WriteString("\n")
	b.WriteString(dim.Render("  Join the Flatcar community: ") + link.Render("https://flatcar.org/discord"))
	b.WriteString("\n")

	return b.String()
}
