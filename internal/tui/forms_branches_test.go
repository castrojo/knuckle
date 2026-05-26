package tui

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"charm.land/huh/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

func forceInterface(v reflect.Value) any {
	if v.CanInterface() {
		return v.Interface()
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface()
}

func formGroups(form *huh.Form) []*huh.Group {
	selector := reflect.ValueOf(form).Elem().FieldByName("selector")
	items := selector.Elem().FieldByName("items")
	groups := make([]*huh.Group, items.Len())
	for i := 0; i < items.Len(); i++ {
		groups[i] = forceInterface(items.Index(i)).(*huh.Group)
	}
	return groups
}

func groupFields(group *huh.Group) []huh.Field {
	selector := reflect.ValueOf(group).Elem().FieldByName("selector")
	items := selector.Elem().FieldByName("items")
	fields := make([]huh.Field, items.Len())
	for i := 0; i < items.Len(); i++ {
		fields[i] = forceInterface(items.Index(i)).(huh.Field)
	}
	return fields
}

func fieldTitle(field any) string {
	v := reflect.ValueOf(field).Elem().FieldByName("title").FieldByName("val")
	return v.String()
}

func mustInputField(t *testing.T, form *huh.Form, title string) *huh.Input {
	t.Helper()
	for _, group := range formGroups(form) {
		for _, field := range groupFields(group) {
			input, ok := field.(*huh.Input)
			if ok && fieldTitle(input) == title {
				return input
			}
		}
	}
	t.Fatalf("input field %q not found", title)
	return nil
}

func mustSelectField(t *testing.T, form *huh.Form, title string) *huh.Select[string] {
	t.Helper()
	for _, group := range formGroups(form) {
		for _, field := range groupFields(group) {
			selectField, ok := field.(*huh.Select[string])
			if ok && fieldTitle(selectField) == title {
				return selectField
			}
		}
	}
	t.Fatalf("select field %q not found", title)
	return nil
}

func selectOptions(selectField *huh.Select[string]) []huh.Option[string] {
	v := reflect.ValueOf(selectField).Elem().FieldByName("options").FieldByName("val")
	return forceInterface(v).([]huh.Option[string])
}

func optionKeys(options []huh.Option[string]) []string {
	keys := make([]string, 0, len(options))
	for _, option := range options {
		keys = append(keys, option.Key)
	}
	return keys
}

func TestBuildUserForm_ValidatorBranches(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		configure func(*Model)
		wantErr   bool
	}{
		{
			name:    "empty hostname allowed",
			title:   "Hostname",
			wantErr: false,
		},
		{
			name:  "invalid hostname rejected",
			title: "Hostname",
			configure: func(m *Model) {
				m.Wizard.State.Config.Hostname = "bad host"
			},
			wantErr: true,
		},
		{
			name:  "valid timezone accepted",
			title: "Timezone",
			configure: func(m *Model) {
				m.Wizard.State.Config.Timezone = "America/New_York"
			},
			wantErr: false,
		},
		{
			name:  "invalid timezone rejected",
			title: "Timezone",
			configure: func(m *Model) {
				m.Wizard.State.Config.Timezone = "Invalid Timezone"
			},
			wantErr: true,
		},
		{
			name:    "empty username allowed",
			title:   "Username",
			wantErr: false,
		},
		{
			name:  "invalid username rejected",
			title: "Username",
			configure: func(m *Model) {
				m.usernameInput = "Admin"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(newTestWizard())
			if tt.configure != nil {
				tt.configure(m)
			}

			field := mustInputField(t, m.buildUserForm(), tt.title)
			field.Blur()

			if gotErr := field.Error() != nil; gotErr != tt.wantErr {
				t.Fatalf("%s validation error = %v, want %v", tt.title, gotErr, tt.wantErr)
			}
		})
	}
}

func TestBuildUserForm_PreservesPopulatedInputs(t *testing.T) {
	m := New(newTestWizard())
	m.Wizard.State.Config.Hostname = "flatcar-node01"
	m.Wizard.State.Config.Timezone = "UTC"
	m.usernameInput = "core"
	m.passwordInput = "s3cr3t"
	m.githubUserInput = "@octocat"
	m.sshKeyInput = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGRlbW8xMjM0NTY3ODkw demo"

	form := m.buildUserForm()

	if got := mustInputField(t, form, "Hostname").GetValue().(string); got != m.Wizard.State.Config.Hostname {
		t.Fatalf("Hostname value = %q, want %q", got, m.Wizard.State.Config.Hostname)
	}
	if got := mustInputField(t, form, "Timezone").GetValue().(string); got != m.Wizard.State.Config.Timezone {
		t.Fatalf("Timezone value = %q, want %q", got, m.Wizard.State.Config.Timezone)
	}
	if got := mustInputField(t, form, "Username").GetValue().(string); got != m.usernameInput {
		t.Fatalf("Username value = %q, want %q", got, m.usernameInput)
	}
	if got := mustInputField(t, form, "Password").GetValue().(string); got != m.passwordInput {
		t.Fatalf("Password value = %q, want %q", got, m.passwordInput)
	}
	if got := mustInputField(t, form, "GitHub Username").GetValue().(string); got != m.githubUserInput {
		t.Fatalf("GitHub Username value = %q, want %q", got, m.githubUserInput)
	}
	if got := mustInputField(t, form, "SSH Public Key").GetValue().(string); got != m.sshKeyInput {
		t.Fatalf("SSH Public Key value = %q, want %q", got, m.sshKeyInput)
	}
}

func TestBuildNetworkForm_ValidatorBranches(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		configure func(*Model)
		wantErr   bool
	}{
		{
			name:    "empty address allowed",
			title:   "IP Address",
			wantErr: false,
		},
		{
			name:  "invalid cidr rejected",
			title: "IP Address",
			configure: func(m *Model) {
				m.Wizard.State.Config.Network.Address = "192.168.1.10"
			},
			wantErr: true,
		},
		{
			name:  "valid cidr accepted",
			title: "IP Address",
			configure: func(m *Model) {
				m.Wizard.State.Config.Network.Address = "192.168.1.10/24"
			},
			wantErr: false,
		},
		{
			name:    "empty gateway allowed",
			title:   "Gateway",
			wantErr: false,
		},
		{
			name:  "invalid gateway rejected",
			title: "Gateway",
			configure: func(m *Model) {
				m.Wizard.State.Config.Network.Gateway = "gateway"
			},
			wantErr: true,
		},
		{
			name:  "valid gateway accepted",
			title: "Gateway",
			configure: func(m *Model) {
				m.Wizard.State.Config.Network.Gateway = "192.168.1.1"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(newTestWizard())
			if tt.configure != nil {
				tt.configure(m)
			}

			field := mustInputField(t, m.buildNetworkForm(), tt.title)
			field.Blur()

			if gotErr := field.Error() != nil; gotErr != tt.wantErr {
				t.Fatalf("%s validation error = %v, want %v", tt.title, gotErr, tt.wantErr)
			}
		})
	}
}

func TestBuildNetworkForm_BuildsInterfaceOptionsAndBindings(t *testing.T) {
	m := New(newTestWizard())
	m.networkModeInput = "static"
	m.dnsInput = "1.1.1.1,8.8.8.8"
	m.Wizard.State.Config.Network.Interface = "eth1"
	m.Wizard.State.Config.Network.Address = "10.0.0.25/24"
	m.Wizard.State.Config.Network.Gateway = "10.0.0.1"
	m.Wizard.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
		{Name: "eth1", MAC: "aa:bb:cc:dd:ee:ff", State: "down"},
	}

	form := m.buildNetworkForm()

	if got := mustSelectField(t, form, "Network Mode").GetValue().(string); got != "static" {
		t.Fatalf("Network Mode value = %q, want static", got)
	}
	if got := mustSelectField(t, form, "Interface").GetValue().(string); got != "eth1" {
		t.Fatalf("Interface value = %q, want eth1", got)
	}
	if got := mustInputField(t, form, "DNS Servers").GetValue().(string); got != m.dnsInput {
		t.Fatalf("DNS Servers value = %q, want %q", got, m.dnsInput)
	}

	keys := optionKeys(selectOptions(mustSelectField(t, form, "Interface")))
	if len(keys) != 3 {
		t.Fatalf("interface option count = %d, want 3", len(keys))
	}
	joined := strings.Join(keys, "\n")
	for _, want := range []string{
		"Auto (DHCP on all interfaces)",
		"eth0 — 00:11:22:33:44:55 (up)",
		"eth1 — aa:bb:cc:dd:ee:ff (down)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("interface options missing %q in %q", want, joined)
		}
	}
}

func TestBuildTailscaleForm_ValidatorBranches(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*Model)
		wantErr   bool
	}{
		{
			name:    "empty auth key allowed",
			wantErr: false,
		},
		{
			name: "invalid auth key rejected",
			configure: func(m *Model) {
				m.tailscaleAuthKeyIn = "tskey-auth-short"
			},
			wantErr: true,
		},
		{
			name: "valid auth key accepted",
			configure: func(m *Model) {
				m.tailscaleAuthKeyIn = "tskey-auth-1234567890-abcdefghijklmnopqrstuvwxyz123456"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(newTestWizard())
			if tt.configure != nil {
				tt.configure(m)
			}

			field := mustInputField(t, m.buildTailscaleForm(), "Auth Key")
			field.Blur()

			if gotErr := field.Error() != nil; gotErr != tt.wantErr {
				t.Fatalf("Auth Key validation error = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestBuildTailscaleForm_PreservesModeAndRoutes(t *testing.T) {
	m := New(newTestWizard())
	m.tailscaleAuthKeyIn = "tskey-auth-1234567890-abcdefghijklmnopqrstuvwxyz123456"
	m.tailscaleModeIn = model.TailscaleModeSubnetRouter
	m.tailscaleRoutesIn = "10.0.0.0/24,192.168.1.0/24"

	form := m.buildTailscaleForm()

	if got := mustSelectField(t, form, "Mode").GetValue().(string); got != model.TailscaleModeSubnetRouter {
		t.Fatalf("Mode value = %q, want %q", got, model.TailscaleModeSubnetRouter)
	}
	if got := mustInputField(t, form, "Advertised routes").GetValue().(string); got != m.tailscaleRoutesIn {
		t.Fatalf("Advertised routes value = %q, want %q", got, m.tailscaleRoutesIn)
	}

	keys := optionKeys(selectOptions(mustSelectField(t, form, "Mode")))
	if len(keys) != 3 {
		t.Fatalf("mode option count = %d, want 3", len(keys))
	}
	joined := strings.Join(keys, "\n")
	for _, want := range []string{
		"Just connect — plain client",
		"Exit node — advertise as exit node",
		"Subnet router — advertise routes",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("mode options missing %q in %q", want, joined)
		}
	}
}
