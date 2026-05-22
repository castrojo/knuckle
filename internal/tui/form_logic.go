package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/projectbluefin/knuckle/internal/github"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// initForm sets up the huh form for form-based steps.
// Non-form steps (Storage, Sysext, Update, Install, Done) set activeForm = nil.
func (m *Model) initForm() {
	switch m.Wizard.State.CurrentStep {
	case model.StepWelcome:
		m.activeForm = nil // Custom card-based channel selector
	case model.StepNetwork:
		m.dnsInput = strings.Join(m.Wizard.State.Config.Network.DNS, ",")
		if m.networkModeInput == "" {
			m.networkModeInput = "dhcp"
		}
		m.activeForm = m.buildNetworkForm()
	case model.StepUser:
		if len(m.Wizard.State.Config.Users) > 0 {
			m.usernameInput = m.Wizard.State.Config.Users[0].Username
		}
		if m.usernameInput == "" {
			m.usernameInput = "core"
		}
		if m.Wizard.State.Config.Hostname == "" {
			m.Wizard.State.Config.Hostname = "flatcar"
		}
		if m.Wizard.State.Config.Timezone == "" {
			m.Wizard.State.Config.Timezone = "UTC"
		}
		m.activeForm = m.buildUserForm()
	case model.StepTailscale:
		m.tailscaleAuthKeyIn = m.Wizard.State.Config.Tailscale.AuthKey
		m.tailscaleModeIn = m.Wizard.State.Config.Tailscale.Mode
		if m.tailscaleModeIn == "" {
			m.tailscaleModeIn = model.TailscaleModeConnect
		}
		m.tailscaleRoutesIn = m.Wizard.State.Config.Tailscale.Routes
		m.activeForm = m.buildTailscaleForm()
	case model.StepReview:
		m.activeForm = m.buildReviewForm()
	default:
		m.activeForm = nil
	}
}

// onFormComplete processes form completion and advances the wizard.
func (m *Model) onFormComplete() tea.Cmd {
	step := m.Wizard.State.CurrentStep
	cfg := &m.Wizard.State.Config

	switch step {
	case model.StepWelcome:
		if err := validate.Channel(cfg.Channel); err != nil {
			m.err = err
			m.initForm()
			if m.activeForm != nil {
				return m.activeForm.Init()
			}
			return nil
		}
		if cfg.IgnitionURL != "" {
			m.Wizard.GoToStep(model.StepStorage)
			m.err = nil
			m.cursor = 0
			m.initStepFields()
			m.initForm()
			if m.activeForm != nil {
				return m.activeForm.Init()
			}
			return nil
		}

	case model.StepNetwork:
		m.Wizard.ApplyNetworkStep(wizard.NetworkStepInput{
			Mode: m.networkModeInput,
			DNS:  m.dnsInput,
		})

	case model.StepUser:
		if err := m.Wizard.ApplyUserStep(wizard.UserStepInput{
			Username:  m.usernameInput,
			Password:  m.passwordInput,
			ManualKey: m.sshKeyInput,
			LocalKeys: detectLocalSSHKeys(),
			Hostname:  cfg.Hostname,
			Timezone:  cfg.Timezone,
		}); err != nil {
			m.err = err
			m.initForm()
			return m.activeForm.Init()
		}
		// Async GitHub key fetch (merges with local + manual keys on return)
		if m.githubUserInput != "" {
			m.fetching = true
			username := strings.TrimPrefix(m.githubUserInput, "@")
			return func() tea.Msg {
				keys, err := github.FetchKeys(username)
				return fetchKeysMsg{keys: keys, err: err}
			}
		}

	case model.StepTailscale:
		cfg.Tailscale.AuthKey = strings.TrimSpace(m.tailscaleAuthKeyIn)
		cfg.Tailscale.Mode = m.tailscaleModeIn
		if cfg.Tailscale.Mode == "" {
			cfg.Tailscale.Mode = model.TailscaleModeConnect
		}
		cfg.Tailscale.Routes = strings.TrimSpace(m.tailscaleRoutesIn)
		if err := m.Wizard.ValidateCurrentStep(); err != nil {
			m.err = err
			m.initForm()
			if m.activeForm != nil {
				return m.activeForm.Init()
			}
			return nil
		}

	case model.StepReview:
		if !m.Wizard.State.Confirmed {
			// User said "Go back"
			m.Wizard.Previous()
			m.err = nil
			m.cursor = 0
			m.initStepFields()
			m.initForm()
			if m.activeForm != nil {
				return m.activeForm.Init()
			}
			return nil
		}
		// Advance to install
		if err := m.Wizard.Next(); err != nil {
			m.err = err
			m.Wizard.State.Confirmed = false // reset so form can be re-answered
			m.initForm()
			return m.activeForm.Init()
		}
		m.err = nil
		m.cursor = 0
		m.initStepFields()
		m.initForm()
		// Start install
		m.installing = true
		return m.startInstall()
	}

	// Advance to next step
	if err := m.Wizard.Next(); err != nil {
		m.err = err
		m.initForm()
		if m.activeForm != nil {
			return m.activeForm.Init()
		}
		return nil
	}
	m.err = nil
	m.cursor = 0
	m.initStepFields()
	m.initForm()
	if m.activeForm != nil {
		return m.activeForm.Init()
	}
	return nil
}

// viewWithForm renders the breadcrumb + form view for form-based steps.
func (m *Model) viewWithForm() string {
	var b strings.Builder

	// Breadcrumb navigation (conversational style)
	b.WriteString(m.buildBreadcrumb())
	b.WriteString("\n")

	// System checks — only on Welcome step, only if warn/fail
	if m.Wizard.State.CurrentStep == model.StepWelcome {
		checksStr := m.renderSystemChecks()
		if checksStr != "" {
			b.WriteString(checksStr)
			b.WriteString("\n")
		}
	}

	// Form
	if m.activeForm != nil {
		b.WriteString(m.activeForm.View())
	}

	// Error
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ %s", m.err.Error())))
		b.WriteString("\n")
	}

	// Fetching indicator
	if m.fetching {
		b.WriteString("\n  ⣾ Fetching SSH keys from GitHub...\n")
	}

	return b.String()
}
