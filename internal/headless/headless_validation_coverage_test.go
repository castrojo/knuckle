package headless

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// Tests covering previously-uncovered validation error branches in Config.Validate()
// and error paths in Run().

func TestValidate_StaticNetworkMissingAddress(t *testing.T) {
	// Covers L257: static mode requires address
	cfg := &Config{
		Channel:  "stable",
		Hostname: "node01",
		Network: NetworkConfig{
			Mode:      "static",
			Interface: "eth0",
			// Address intentionally omitted
			Gateway: "192.168.1.1",
		},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/sda",
		UpdateStrategy: "reboot",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing address in static mode")
	}
	if !strings.Contains(err.Error(), "static mode requires address") {
		t.Errorf("expected 'static mode requires address' in error, got: %v", err)
	}
}

func TestValidate_UserEmptyUsername(t *testing.T) {
	// Covers L297: users[i] username is required
	cfg := &Config{
		Channel:  "stable",
		Hostname: "node01",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users: []UserConfig{
			{Username: "", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}},
		},
		Disk:           "/dev/vdb",
		UpdateStrategy: "reboot",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty username")
	}
	if !strings.Contains(err.Error(), "username is required") {
		t.Errorf("expected 'username is required' in error, got: %v", err)
	}
}

func TestValidate_UserInvalidUsernameFormat(t *testing.T) {
	// Covers L300: validate.Username returns error for invalid format
	cfg := &Config{
		Channel:  "stable",
		Hostname: "node01",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users: []UserConfig{
			{Username: "123-INVALID", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}},
		},
		Disk:           "/dev/vdb",
		UpdateStrategy: "reboot",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid username format")
	}
	if !strings.Contains(err.Error(), "users[0]") {
		t.Errorf("expected 'users[0]' in error, got: %v", err)
	}
}

func TestRun_GitHubKeyInvalidSSHKeyReturned(t *testing.T) {
	// Covers L421: validate.SSHPublicKey fails on a key from GitHub
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()

	fetchGitHubKeysFunc = func(ctx context.Context, username string) ([]string, error) {
		return []string{"not-a-valid-ssh-key"}, nil
	}

	cfg := &Config{
		Channel:        "stable",
		Hostname:       "node01",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", GithubUser: "testuser"}},
		Disk:           "/dev/vdb",
		DryRun:         true,
		UpdateStrategy: "reboot",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := Run(context.Background(), cfg, &mockInstaller{}, logger)
	if err == nil {
		t.Fatal("expected error for invalid SSH key from GitHub")
	}
	if !strings.Contains(err.Error(), "invalid SSH key from GitHub user") {
		t.Errorf("expected 'invalid SSH key from GitHub user' in error, got: %v", err)
	}
}

func TestRun_RebootWithContextCancellation(t *testing.T) {
	// Covers L477-485: reboot path when context is cancelled (Reboot=true, DryRun=false)
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(ctx context.Context, username string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign", // bypass disk and user requirements
		Reboot:      true,
		DryRun:      false, // must be false to enter reboot branch
	}

	// Cancel context immediately so the select in reboot catches ctx.Done()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use a fast installer that skips block device checks
	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	err := Run(ctx, cfg, installer, logger)
	// Should get context.Canceled either from installer or reboot timer
	if err == nil {
		t.Fatal("expected error from cancelled context during reboot wait")
	}
}

func TestRun_RebootDryRunMessage(t *testing.T) {
	// Covers L496: "reboot skipped — dry-run mode" message path
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(ctx context.Context, username string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:        "stable",
		Hostname:       "node01",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/vdb",
		Reboot:         true,
		DryRun:         true,
		UpdateStrategy: "reboot",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := Run(context.Background(), cfg, &mockInstaller{}, logger)
	if err != nil {
		t.Fatalf("expected no error for dry-run reboot path, got: %v", err)
	}
}

// skipBlockDeviceInstaller always succeeds regardless of context state,
// allowing subsequent reboot logic to run.
type skipBlockDeviceInstaller struct{}

func (s *skipBlockDeviceInstaller) Install(_ context.Context, _ *model.InstallConfig, progress func(string)) error {
	progress("mock install")
	return nil
}

