package tui

import (
	"strings"
	"testing"
)

func TestValidateOptionalCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid cidr", input: "192.168.1.10/24", wantErr: false},
		{name: "invalid cidr", input: "192.168.1.10", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateOptionalCIDR(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateOptionalCIDR(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateOptionalIPAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid ipv4", input: "192.168.1.1", wantErr: false},
		{name: "invalid ipv4", input: "gateway", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateOptionalIPAddress(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateOptionalIPAddress(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateOptionalHostname(t *testing.T) {
	tooLong := strings.Repeat("a", 64)
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid hostname", input: "flatcar-node01", wantErr: false},
		{name: "invalid hostname", input: tooLong, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateOptionalHostname(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateOptionalHostname(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateTimezoneInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid timezone", input: "America/New_York", wantErr: false},
		{name: "invalid timezone", input: "Bad Timezone", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateTimezoneInput(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateTimezoneInput(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateOptionalUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid username", input: "core", wantErr: false},
		{name: "invalid username", input: "Core", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateOptionalUsername(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateOptionalUsername(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateOptionalTailscaleAuthKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed", input: "", wantErr: false},
		{name: "valid auth key", input: "tskey-auth-1234567890-abcdefghijklmnopqrstuvwxyz123456", wantErr: false},
		{name: "invalid auth key", input: "tskey-auth-short", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateOptionalTailscaleAuthKey(tt.input) != nil; gotErr != tt.wantErr {
				t.Fatalf("validateOptionalTailscaleAuthKey(%q) error = %v, wantErr %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}
