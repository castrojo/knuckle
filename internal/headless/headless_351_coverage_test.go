package headless

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestToInstallConfig_AlwaysBuildsInstallConfig verifies ToInstallConfig always
// produces an InstallConfig for validated input shapes.
func TestToInstallConfig_AlwaysBuildsInstallConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
	}{
		{"minimal", &Config{}},
		{"empty strings", &Config{Channel: "", Hostname: "", Disk: ""}},
		{"full config", &Config{
			Channel:        "stable",
			Hostname:       "node01",
			Timezone:       "America/New_York",
			Arch:           "arm64",
			Network:        NetworkConfig{Mode: "static", Interface: "eth0", Address: "10.0.0.2/24", Gateway: "10.0.0.1"},
			Users:          []UserConfig{{Username: "admin", SSHKeys: []string{"ssh-ed25519 AAAA k"}, Groups: []string{"wheel"}}},
			Disk:           "/dev/nvme0n1",
			UpdateStrategy: "off",
			Tailscale:      TailscaleConfig{AuthKey: "tskey-auth-k123-secret", Mode: "exit-node"},
			Swap:           &SwapConfig{Enabled: true, SizeMB: 8192},
		}},
		{"nil swap", &Config{Channel: "stable", Users: []UserConfig{{Username: "core"}}}},
		{"disabled swap", &Config{Channel: "stable", Swap: &SwapConfig{Enabled: false}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ic := tc.cfg.ToInstallConfig()
			if ic == nil {
				t.Fatal("ToInstallConfig returned nil config")
			}
		})
	}
}

// TestRun_ToInstallConfigPathExercised verifies the full Run path passes through
// config conversion and into install execution.
func TestRun_ToInstallConfigPathExercised(t *testing.T) {
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign",
		Disk:        "/dev/sda",
		DryRun:      true,
	}

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(context.Background(), cfg, installer, logger)
	if err != nil {
		t.Fatalf("Run should succeed for valid config: %v", err)
	}
}

// TestRun_InvalidGitHubKeyFromRemote_BlocksInstall ensures that when GitHub
// returns a key that fails SSH validation, the installer is never called.
// This covers the error path at line 421-423 from a "blocksInstall" perspective.
func TestRun_InvalidGitHubKeyFromRemote_BlocksInstall(t *testing.T) {
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{"invalid-key-format"}, nil
	}

	cfg := &Config{
		Channel:  "stable",
		Hostname: "node01",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users:    []UserConfig{{Username: "core", GithubUser: "badkeys"}},
		Disk:     "/dev/vdb",
		DryRun:   true,
	}

	mock := &mockInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(context.Background(), cfg, mock, logger)
	if err == nil {
		t.Fatal("expected error for invalid GitHub SSH key")
	}
	if mock.called {
		t.Error("installer should NOT be called when GitHub key validation fails")
	}
}

// TestResolveSysexts_NotFoundInCatalog covers the error path at line 204
// when a requested sysext name doesn't exist in the catalog index.
func TestResolveSysexts_NotFoundInCatalog(t *testing.T) {
	catalog := []model.SysextEntry{
		{Name: "docker", Version: "24.0", URL: "https://example.com/docker.raw"},
	}

	// Create a mock bakery client that returns this catalog
	origBakery := newBakeryClientFunc
	defer func() { newBakeryClientFunc = origBakery }()

	_, err := resolveSysexts(context.Background(), []string{"nonexistent"}, &mockBakeryClientForResolve{entries: catalog}, "amd64")
	if err == nil {
		t.Fatal("expected error for sysext not found in catalog")
	}
	if got := err.Error(); got != `sysext "nonexistent" not found in catalog` {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockBakeryClientForResolve struct {
	entries []model.SysextEntry
}

func (m *mockBakeryClientForResolve) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return m.entries, nil
}

func (m *mockBakeryClientForResolve) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	return m.entries, nil
}

func (m *mockBakeryClientForResolve) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	return m.entries, nil
}
