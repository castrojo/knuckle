package validate

import (
	"strings"
	"testing"
)

func TestHostname(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myhost", false},
		{"valid with hyphen", "my-host", false},
		{"valid with numbers", "host123", false},
		{"valid single char", "a", false},
		{"valid mixed case", "MyHost", false},
		{"valid max length", strings.Repeat("a", 63), false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 64), true},
		{"leading hyphen", "-host", true},
		{"trailing hyphen", "host-", true},
		{"contains dot", "my.host", true},
		{"contains space", "my host", true},
		{"contains underscore", "my_host", true},
		{"only hyphen", "-", true},
		{"special chars", "host!", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Hostname(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hostname(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestIPAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "192.168.1.1", false},
		{"valid loopback", "127.0.0.1", false},
		{"valid zeros", "0.0.0.0", false},
		{"valid broadcast", "255.255.255.255", false},
		{"invalid empty", "", true},
		{"invalid text", "notanip", true},
		{"invalid octet", "192.168.1.256", true},
		{"ipv6 rejected", "::1", true},
		{"ipv6 full rejected", "2001:db8::1", true},
		{"cidr not allowed", "192.168.1.1/24", true},
		{"trailing dot", "192.168.1.1.", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IPAddress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("IPAddress(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid /24", "192.168.1.0/24", false},
		{"valid /32", "10.0.0.1/32", false},
		{"valid /8", "10.0.0.0/8", false},
		{"valid host in subnet", "192.168.1.100/24", false},
		{"invalid no mask", "192.168.1.0", true},
		{"invalid empty", "", true},
		{"invalid text", "notacidr", true},
		{"invalid mask", "192.168.1.0/33", true},
		{"ipv6 rejected", "2001:db8::/32", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CIDR(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDR(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSSHPublicKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid ed25519", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host", false},
		{"valid rsa", "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQExample user@host", false},
		{"valid ecdsa", "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY= comment", false},
		{"valid no comment", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample", false},
		{"valid sk key", "sk-ssh-ed25519@openssh.com AAAAG3NrLXNzaC1lZDI1NTE5 user", false},
		{"invalid empty", "", true},
		{"invalid no data", "ssh-ed25519", true},
		{"invalid type", "ssh-invalid AAAAC3NzaC1lZDI1NTE5AAAAIExample", true},
		{"invalid just text", "notakey", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SSHPublicKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SSHPublicKey(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "jorge", false},
		{"valid underscore start", "_admin", false},
		{"valid with hyphen", "my-user", false},
		{"valid with numbers", "user01", false},
		{"valid underscore", "my_user", false},
		{"valid single char", "a", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 33), true},
		{"starts with number", "1user", true},
		{"starts with hyphen", "-user", true},
		{"uppercase", "Admin", true},
		{"contains dot", "my.user", true},
		{"contains space", "my user", true},
		{"special chars", "user!", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Username(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Username(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestDiskPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid sda", "/dev/sda", false},
		{"valid nvme", "/dev/nvme0n1", false},
		{"valid by-id", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK", false},
		{"valid vda", "/dev/vda", false},
		{"invalid empty", "", true},
		{"invalid no prefix", "sda", true},
		{"invalid just prefix", "/dev/", true},
		{"invalid relative", "dev/sda", true},
		{"invalid other path", "/sys/block/sda", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DiskPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiskPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestChannel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"stable", "stable", false},
		{"beta", "beta", false},
		{"alpha", "alpha", false},
		{"edge", "edge", false},
		{"invalid empty", "", true},
		{"invalid uppercase", "Stable", true},
		{"invalid unknown", "nightly", true},
		{"invalid lts", "lts", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Channel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Channel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com/path", false},
		{"valid https with port", "https://example.com:8080/api", false},
		{"invalid empty", "", true},
		{"invalid no scheme", "example.com", true},
		{"invalid ftp", "ftp://example.com", true},
		{"invalid just text", "notaurl", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := URL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("URL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		wantErr bool
	}{
		{"valid", "name", "hello", false},
		{"valid with spaces", "name", "  hello  ", false},
		{"empty string", "name", "", true},
		{"only spaces", "name", "   ", true},
		{"only tabs", "name", "\t\t", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NonEmpty(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NonEmpty(%q, %q) error = %v, wantErr %v", tt.field, tt.value, err, tt.wantErr)
			}
		})
	}
}
