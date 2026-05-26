package tui

import (
	"reflect"
	"testing"
	"unsafe"

	"charm.land/huh/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestBuildNetworkForm_StateCoverageAndValidators(t *testing.T) {
	tests := []struct {
		name             string
		interfaces       []model.NetworkInterface
		networkMode      string
		address          string
		gateway          string
		dns              string
		selectedIface    string
		wantGroupCount   int
		wantInterfaceOps int
	}{
		{
			name:             "nil interfaces",
			interfaces:       nil,
			wantGroupCount:   2,
			wantInterfaceOps: 1,
		},
		{
			name:             "empty interfaces slice",
			interfaces:       []model.NetworkInterface{},
			networkMode:      model.NetworkDHCP.String(),
			wantGroupCount:   2,
			wantInterfaceOps: 1,
		},
		{
			name: "populated static network",
			interfaces: []model.NetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
				{Name: "wlan0", MAC: "AA:BB:CC:DD:EE:FF", State: "down"},
			},
			networkMode:      model.NetworkStatic.String(),
			address:          "192.168.1.100/24",
			gateway:          "192.168.1.1",
			dns:              "1.1.1.1,8.8.8.8",
			selectedIface:    "eth0",
			wantGroupCount:   2,
			wantInterfaceOps: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			w.State.Interfaces = tt.interfaces
			m := New(w)
			m.networkModeInput = tt.networkMode
			m.Wizard.State.Config.Network.Address = tt.address
			m.Wizard.State.Config.Network.Gateway = tt.gateway
			m.Wizard.State.Config.Network.Interface = tt.selectedIface
			m.dnsInput = tt.dns

			form := m.buildNetworkForm()
			if form == nil {
				t.Fatal("buildNetworkForm returned nil")
			}
			if got := len(formGroups(t, form)); got != tt.wantGroupCount {
				t.Fatalf("group count = %d, want %d", got, tt.wantGroupCount)
			}
			if got := len(selectOptions(t, mustFindSelect(t, form, "Interface"))); got != tt.wantInterfaceOps {
				t.Fatalf("interface options = %d, want %d", got, tt.wantInterfaceOps)
			}
			if got := m.Wizard.State.Config.Network.Interface; got != tt.selectedIface {
				t.Fatalf("interface = %q, want %q", got, tt.selectedIface)
			}
			if got := m.Wizard.State.Config.Network.Address; got != tt.address {
				t.Fatalf("address = %q, want %q", got, tt.address)
			}
			if got := m.Wizard.State.Config.Network.Gateway; got != tt.gateway {
				t.Fatalf("gateway = %q, want %q", got, tt.gateway)
			}
			if got := m.dnsInput; got != tt.dns {
				t.Fatalf("dnsInput = %q, want %q", got, tt.dns)
			}
		})
	}

	form := New(newTestWizard()).buildNetworkForm()
	cidrValidator := inputValidator(t, mustFindInput(t, form, "IP Address"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty CIDR allowed", input: "", wantErr: false},
		{name: "valid CIDR", input: "10.0.0.5/24", wantErr: false},
		{name: "invalid CIDR", input: "10.0.0.5", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := cidrValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("CIDR validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}

	gatewayValidator := inputValidator(t, mustFindInput(t, form, "Gateway"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty gateway allowed", input: "", wantErr: false},
		{name: "valid IPv4", input: "192.168.1.1", wantErr: false},
		{name: "invalid IPv4", input: "not-an-ip", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := gatewayValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("gateway validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestBuildUserForm_StateCoverageAndValidators(t *testing.T) {
	tests := []struct {
		name           string
		hostname       string
		timezone       string
		username       string
		password       string
		githubUser     string
		sshKey         string
		wantGroupCount int
	}{
		{
			name:           "empty user inputs",
			wantGroupCount: 2,
		},
		{
			name:           "populated user inputs",
			hostname:       "flatcar-node01",
			timezone:       "America/New_York",
			username:       "admin",
			password:       "secret123",
			githubUser:     "@octocat",
			sshKey:         "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyComment user@example",
			wantGroupCount: 2,
		},
		{
			name:           "missing ssh auth inputs",
			hostname:       "edge-node",
			timezone:       "UTC",
			username:       "core",
			wantGroupCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			m := New(w)
			m.Wizard.State.Config.Hostname = tt.hostname
			m.Wizard.State.Config.Timezone = tt.timezone
			m.usernameInput = tt.username
			m.passwordInput = tt.password
			m.githubUserInput = tt.githubUser
			m.sshKeyInput = tt.sshKey

			form := m.buildUserForm()
			if form == nil {
				t.Fatal("buildUserForm returned nil")
			}
			if got := len(formGroups(t, form)); got != tt.wantGroupCount {
				t.Fatalf("group count = %d, want %d", got, tt.wantGroupCount)
			}
			for _, title := range []string{"Hostname", "Timezone", "Username", "Password", "GitHub Username", "SSH Public Key"} {
				if mustFindInput(t, form, title) == nil {
					t.Fatalf("missing input %q", title)
				}
			}
			if got := m.Wizard.State.Config.Hostname; got != tt.hostname {
				t.Fatalf("hostname = %q, want %q", got, tt.hostname)
			}
			if got := m.Wizard.State.Config.Timezone; got != tt.timezone {
				t.Fatalf("timezone = %q, want %q", got, tt.timezone)
			}
			if got := m.usernameInput; got != tt.username {
				t.Fatalf("username = %q, want %q", got, tt.username)
			}
			if got := m.passwordInput; got != tt.password {
				t.Fatalf("password = %q, want %q", got, tt.password)
			}
			if got := m.githubUserInput; got != tt.githubUser {
				t.Fatalf("github user = %q, want %q", got, tt.githubUser)
			}
			if got := m.sshKeyInput; got != tt.sshKey {
				t.Fatalf("ssh key = %q, want %q", got, tt.sshKey)
			}
		})
	}

	form := New(newTestWizard()).buildUserForm()
	hostnameValidator := inputValidator(t, mustFindInput(t, form, "Hostname"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty hostname allowed", input: "", wantErr: false},
		{name: "valid hostname", input: "flatcar-node01", wantErr: false},
		{name: "invalid hostname", input: "bad_host", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := hostnameValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("hostname validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}

	usernameValidator := inputValidator(t, mustFindInput(t, form, "Username"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty username allowed", input: "", wantErr: false},
		{name: "valid username", input: "admin", wantErr: false},
		{name: "invalid username", input: "0admin", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := usernameValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("username validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}

	timezoneValidator := inputValidator(t, mustFindInput(t, form, "Timezone"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty timezone allowed", input: "", wantErr: false},
		{name: "valid timezone", input: "Europe/Berlin", wantErr: false},
		{name: "invalid timezone", input: "bad timezone", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := timezoneValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("timezone validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestBuildTailscaleForm_StateCoverageAndValidators(t *testing.T) {
	tests := []struct {
		name           string
		authKey        string
		mode           string
		routes         string
		wantGroupCount int
	}{
		{
			// huh.Select initializes the bound value to the first option when empty.
			// First option is TailscaleModeConnect — expect that as the post-build value.
			name:           "empty tailscale config",
			mode:           model.TailscaleModeConnect,
			wantGroupCount: 1,
		},
		{
			name:           "exit node config",
			authKey:        "tskey-auth-ABCDEFGHIJ-12345678901234567890",
			mode:           model.TailscaleModeExitNode,
			wantGroupCount: 1,
		},
		{
			name:           "subnet router config",
			authKey:        "tskey-auth-KNUCKLE1234-abcdefghijklmnopqrstuvwxyz12",
			mode:           model.TailscaleModeSubnetRouter,
			routes:         "10.0.0.0/24,192.168.1.0/24",
			wantGroupCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(newTestWizard())
			m.tailscaleAuthKeyIn = tt.authKey
			m.tailscaleModeIn = tt.mode
			m.tailscaleRoutesIn = tt.routes

			form := m.buildTailscaleForm()
			if form == nil {
				t.Fatal("buildTailscaleForm returned nil")
			}
			if got := len(formGroups(t, form)); got != tt.wantGroupCount {
				t.Fatalf("group count = %d, want %d", got, tt.wantGroupCount)
			}
			if got := m.tailscaleAuthKeyIn; got != tt.authKey {
				t.Fatalf("auth key = %q, want %q", got, tt.authKey)
			}
			if got := m.tailscaleModeIn; got != tt.mode {
				t.Fatalf("mode = %q, want %q", got, tt.mode)
			}
			if got := m.tailscaleRoutesIn; got != tt.routes {
				t.Fatalf("routes = %q, want %q", got, tt.routes)
			}
		})
	}

	form := New(newTestWizard()).buildTailscaleForm()
	authKeyValidator := inputValidator(t, mustFindInput(t, form, "Auth Key"))
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty auth key allowed", input: "", wantErr: false},
		{name: "valid auth key", input: "tskey-auth-ABCDEFGHIJ-12345678901234567890", wantErr: false},
		{name: "invalid auth key", input: "not-a-tskey", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := authKeyValidator(tc.input); (err != nil) != tc.wantErr {
				t.Fatalf("auth key validator(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func mustFindInput(t *testing.T, form *huh.Form, title string) *huh.Input {
	t.Helper()
	for _, group := range formGroups(t, form) {
		for _, field := range groupFields(t, group) {
			input, ok := field.(*huh.Input)
			if ok && fieldTitle(input) == title {
				return input
			}
		}
	}
	t.Fatalf("input %q not found", title)
	return nil
}

func mustFindSelect(t *testing.T, form *huh.Form, title string) *huh.Select[string] {
	t.Helper()
	for _, group := range formGroups(t, form) {
		for _, field := range groupFields(t, group) {
			selectField, ok := field.(*huh.Select[string])
			if ok && fieldTitle(selectField) == title {
				return selectField
			}
		}
	}
	t.Fatalf("select %q not found", title)
	return nil
}

func inputValidator(t *testing.T, input *huh.Input) func(string) error {
	t.Helper()
	value := accessibleValue(reflect.ValueOf(input).Elem().FieldByName("validate"))
	validator, ok := value.Interface().(func(string) error)
	if !ok {
		t.Fatal("input validator has unexpected type")
	}
	return validator
}

func fieldTitle(field any) string {
	value := reflect.ValueOf(field).Elem()
	title := accessibleValue(value.FieldByName("title"))
	return accessibleValue(title.FieldByName("val")).String()
}

func formGroups(t *testing.T, form *huh.Form) []*huh.Group {
	t.Helper()
	selector := accessibleValue(reflect.ValueOf(form).Elem().FieldByName("selector"))
	items := accessibleValue(selector.Elem().FieldByName("items"))
	groups := make([]*huh.Group, items.Len())
	for i := 0; i < items.Len(); i++ {
		group, ok := items.Index(i).Interface().(*huh.Group)
		if !ok {
			t.Fatalf("group %d has unexpected type %T", i, items.Index(i).Interface())
		}
		groups[i] = group
	}
	return groups
}

func groupFields(t *testing.T, group *huh.Group) []huh.Field {
	t.Helper()
	selector := accessibleValue(reflect.ValueOf(group).Elem().FieldByName("selector"))
	items := accessibleValue(selector.Elem().FieldByName("items"))
	fields := make([]huh.Field, items.Len())
	for i := 0; i < items.Len(); i++ {
		field, ok := items.Index(i).Interface().(huh.Field)
		if !ok {
			t.Fatalf("field %d has unexpected type %T", i, items.Index(i).Interface())
		}
		fields[i] = field
	}
	return fields
}

func selectOptions(t *testing.T, field *huh.Select[string]) []reflect.Value {
	t.Helper()
	options := accessibleValue(accessibleValue(reflect.ValueOf(field).Elem().FieldByName("options")).FieldByName("val"))
	result := make([]reflect.Value, options.Len())
	for i := 0; i < options.Len(); i++ {
		result[i] = options.Index(i)
	}
	return result
}

func accessibleValue(value reflect.Value) reflect.Value {
	if value.CanAddr() {
		return reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem()
	}
	return value
}
