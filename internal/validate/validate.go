// Package validate provides input validation functions for the knuckle TUI installer.
package validate

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Hostname validates a Linux hostname (RFC 1123).
// Must be 1-63 characters, alphanumeric plus hyphens, no leading/trailing hyphen, no dots.
func Hostname(s string) error {
	if s == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if len(s) > 63 {
		return fmt.Errorf("hostname too long (max 63 characters)")
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`, s)
	if !matched {
		return fmt.Errorf("invalid hostname %q: must be alphanumeric with optional hyphens, no leading/trailing hyphen", s)
	}
	return nil
}

// IPAddress validates an IPv4 address (without CIDR).
func IPAddress(s string) error {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid IPv4 address: %s", s)
	}
	return nil
}

// CIDR validates an IPv4 CIDR notation (e.g., 192.168.1.10/24).
func CIDR(s string) error {
	ip, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", s)
	}
	if ip.To4() == nil {
		return fmt.Errorf("invalid CIDR: only IPv4 is supported")
	}
	return nil
}

// Gateway validates a gateway IPv4 address.
func Gateway(s string) error {
	return IPAddress(s)
}

// DNSServer validates a DNS server address.
func DNSServer(s string) error {
	return IPAddress(s)
}

// SSHPublicKey validates an SSH public key format.
// Must have at least "type base64data"; an optional comment is allowed.
func SSHPublicKey(s string) error {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return fmt.Errorf("invalid SSH key format")
	}
	validTypes := []string{
		"ssh-rsa",
		"ssh-ed25519",
		"ssh-dss",
		"ecdsa-sha2-nistp256",
		"ecdsa-sha2-nistp384",
		"ecdsa-sha2-nistp521",
		"sk-ssh-ed25519@openssh.com",
		"sk-ecdsa-sha2-nistp256@openssh.com",
	}
	for _, t := range validTypes {
		if parts[0] == t {
			return nil
		}
	}
	return fmt.Errorf("unsupported SSH key type: %s", parts[0])
}

// Username validates a Linux username.
// Must be 1-32 characters, start with a letter or underscore, contain only
// lowercase alphanumeric, underscore, or hyphen.
func Username(s string) error {
	if s == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if len(s) > 32 {
		return fmt.Errorf("username too long (max 32 characters)")
	}
	matched, _ := regexp.MatchString(`^[a-z_][a-z0-9_-]*$`, s)
	if !matched {
		return fmt.Errorf("invalid username: must start with letter or underscore, contain only lowercase alphanumeric, underscore, or hyphen")
	}
	return nil
}

// DiskPath validates a disk device path.
// Must start with /dev/ and have at least one character after the prefix.
func DiskPath(s string) error {
	if !strings.HasPrefix(s, "/dev/") {
		return fmt.Errorf("disk path must start with /dev/")
	}
	if len(s) <= 5 {
		return fmt.Errorf("disk path too short")
	}
	return nil
}

// Channel validates a Flatcar release channel.
func Channel(s string) error {
	valid := []string{"stable", "beta", "alpha", "edge"}
	for _, v := range valid {
		if s == v {
			return nil
		}
	}
	return fmt.Errorf("invalid channel %q: must be one of stable, beta, alpha, edge", s)
}

// URL validates a basic URL format (must start with http:// or https://).
func URL(s string) error {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	return nil
}

// NonEmpty validates that a string is not empty after trimming whitespace.
func NonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", field)
	}
	return nil
}
