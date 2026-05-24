package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestInitFormDefaults_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		step  model.WizardStep
		seed  func(*Model)
		check func(*testing.T, *Model)
	}{
		{
			name: "network joins dns and defaults mode",
			step: model.StepNetwork,
			seed: func(m *Model) {
				m.Wizard.State.Config.Network.DNS = []string{"1.1.1.1", "8.8.8.8"}
				m.networkModeInput = ""
			},
			check: func(t *testing.T, m *Model) {
				t.Helper()
				if m.dnsInput != "1.1.1.1,8.8.8.8" {
					t.Fatalf("dnsInput = %q, want joined DNS list", m.dnsInput)
				}
				if m.networkModeInput != "dhcp" {
					t.Fatalf("networkModeInput = %q, want dhcp", m.networkModeInput)
				}
				if m.activeForm == nil {
					t.Fatal("expected network form to be initialized")
				}
			},
		},
		{
			name: "user populates defaults when empty",
			step: model.StepUser,
			seed: func(m *Model) {
				m.usernameInput = ""
				m.Wizard.State.Config.Hostname = ""
				m.Wizard.State.Config.Timezone = ""
				m.Wizard.State.Config.Users = nil
			},
			check: func(t *testing.T, m *Model) {
				t.Helper()
				if m.usernameInput != "core" {
					t.Fatalf("usernameInput = %q, want core", m.usernameInput)
				}
				if m.Wizard.State.Config.Hostname != "flatcar" {
					t.Fatalf("hostname = %q, want flatcar", m.Wizard.State.Config.Hostname)
				}
				if m.Wizard.State.Config.Timezone != "UTC" {
					t.Fatalf("timezone = %q, want UTC", m.Wizard.State.Config.Timezone)
				}
				if m.activeForm == nil {
					t.Fatal("expected user form to be initialized")
				}
			},
		},
		{
			name: "user keeps existing values",
			step: model.StepUser,
			seed: func(m *Model) {
				m.Wizard.State.Config.Users = []model.UserConfig{{Username: "operator"}}
				m.Wizard.State.Config.Hostname = "node-01"
				m.Wizard.State.Config.Timezone = "America/New_York"
				m.usernameInput = ""
			},
			check: func(t *testing.T, m *Model) {
				t.Helper()
				if m.usernameInput != "operator" {
					t.Fatalf("usernameInput = %q, want operator", m.usernameInput)
				}
				if m.Wizard.State.Config.Hostname != "node-01" {
					t.Fatalf("hostname = %q, want node-01", m.Wizard.State.Config.Hostname)
				}
				if m.Wizard.State.Config.Timezone != "America/New_York" {
					t.Fatalf("timezone = %q, want America/New_York", m.Wizard.State.Config.Timezone)
				}
			},
		},
		{
			name: "tailscale copies values and defaults mode",
			step: model.StepTailscale,
			seed: func(m *Model) {
				m.Wizard.State.Config.Tailscale.AuthKey = "tskey-auth-k1234567890-abcdefghijklmnopqrstuvwxyz123456"
				m.Wizard.State.Config.Tailscale.Mode = ""
				m.Wizard.State.Config.Tailscale.Routes = "10.0.0.0/24"
			},
			check: func(t *testing.T, m *Model) {
				t.Helper()
				if m.tailscaleAuthKeyIn != m.Wizard.State.Config.Tailscale.AuthKey {
					t.Fatalf("tailscaleAuthKeyIn = %q, want copied auth key", m.tailscaleAuthKeyIn)
				}
				if m.tailscaleModeIn != model.TailscaleModeConnect {
					t.Fatalf("tailscaleModeIn = %q, want %q", m.tailscaleModeIn, model.TailscaleModeConnect)
				}
				if m.tailscaleRoutesIn != "10.0.0.0/24" {
					t.Fatalf("tailscaleRoutesIn = %q, want preserved routes", m.tailscaleRoutesIn)
				}
				if m.activeForm == nil {
					t.Fatal("expected tailscale form to be initialized")
				}
			},
		},
		{
			name: "welcome keeps form nil",
			step: model.StepWelcome,
			seed: func(m *Model) {},
			check: func(t *testing.T, m *Model) {
				t.Helper()
				if m.activeForm != nil {
					t.Fatal("expected welcome step to keep activeForm nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			w.State.CurrentStep = tt.step
			m := New(w)
			tt.seed(m)
			m.initForm()
			tt.check(t, m)
		})
	}
}

func TestFormFieldTransforms_TableDriven(t *testing.T) {
	sshTests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "trims whitespace and drops empties",
			input: "  ssh-ed25519 AAAA user@example ; ; ssh-rsa BBBB dev+lab@example  ; ",
			want:  []string{"ssh-ed25519 AAAA user@example", "ssh-rsa BBBB dev+lab@example"},
		},
	}

	for _, tt := range sshTests {
		t.Run("split ssh keys/"+tt.name, func(t *testing.T) {
			if got := splitSSHKeys(tt.input); strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Fatalf("splitSSHKeys(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	passwordTests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty input", input: "", wantErr: false},
		{name: "max length", input: strings.Repeat("a", 72), wantErr: false},
		{name: "too long", input: strings.Repeat("é", 37), wantErr: true},
	}

	for _, tt := range passwordTests {
		t.Run("hash password/"+tt.name, func(t *testing.T) {
			got, err := hashPassword(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("hashPassword(%q) returned nil error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("hashPassword(%q) error = %v", tt.input, err)
			}
			if got == "" {
				t.Fatalf("hashPassword(%q) returned empty hash", tt.input)
			}
		})
	}
}

func TestOnFormCompleteTailscale_TrimsAndDefaults(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	m := New(w)
	m.tailscaleAuthKeyIn = "   "
	m.tailscaleModeIn = ""
	m.tailscaleRoutesIn = " 10.0.0.0/24, 192.168.0.0/24  "

	_ = m.onFormComplete()

	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
	if got := m.Wizard.State.Config.Tailscale.AuthKey; got != "" {
		t.Fatalf("AuthKey = %q, want empty string", got)
	}
	if got := m.Wizard.State.Config.Tailscale.Mode; got != model.TailscaleModeConnect {
		t.Fatalf("Mode = %q, want %q", got, model.TailscaleModeConnect)
	}
	if got := m.Wizard.State.Config.Tailscale.Routes; got != "10.0.0.0/24, 192.168.0.0/24" {
		t.Fatalf("Routes = %q, want trimmed routes", got)
	}
}

func TestViewWithForm_ShowsErrorAndFetchingIndicators(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	m := New(w)
	m.initForm()
	m.err = errors.New("bad input")
	m.fetching = true

	out := m.viewWithForm()

	if !strings.Contains(out, "bad input") {
		t.Fatalf("viewWithForm() missing error text: %q", out)
	}
	if !strings.Contains(out, "Fetching SSH keys from GitHub") {
		t.Fatalf("viewWithForm() missing fetching indicator: %q", out)
	}
}
