package install

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func testFCOSLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestFCOSInstall_GeneratedConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testFCOSLogger())

	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// coreos-installer must be called, not flatcar-install
	var coreosCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "flatcar-install" {
			t.Error("flatcar-install must not be called by FCOSInstaller")
		}
		if spy.Calls[i].Name == "coreos-installer" {
			coreosCall = &spy.Calls[i]
		}
	}
	if coreosCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	// wipefs must NOT be called
	for _, call := range spy.Calls {
		if call.Name == "wipefs" {
			t.Error("wipefs must not be called by FCOSInstaller")
		}
	}

	// sfdisk must NOT be called
	for _, call := range spy.Calls {
		if call.Name == "sfdisk" {
			t.Error("sfdisk must not be called by FCOSInstaller")
		}
	}

	// First arg must be "install" subcommand
	if len(coreosCall.Args) == 0 || coreosCall.Args[0] != "install" {
		t.Fatalf("expected first arg 'install', got %v", coreosCall.Args)
	}

	// Must have --stream stable
	if !argsContainPair(coreosCall.Args, "--stream", "stable") {
		t.Errorf("expected --stream stable in args: %v", coreosCall.Args)
	}

	// Must have --ignition-file <temp path>
	ignIdx := argIndex(coreosCall.Args, "--ignition-file")
	if ignIdx < 0 || ignIdx+1 >= len(coreosCall.Args) {
		t.Fatalf("expected --ignition-file in args: %v", coreosCall.Args)
	}

	// Disk path must be the last (positional) argument
	last := coreosCall.Args[len(coreosCall.Args)-1]
	if last != "/dev/sda" {
		t.Errorf("expected disk path /dev/sda as last arg, got %q", last)
	}
}

func TestFCOSInstall_StreamChannel(t *testing.T) {
	for _, channel := range []string{"stable", "testing", "next"} {
		t.Run(channel, func(t *testing.T) {
			spy := runner.NewSpyRunner()
			installer := NewFCOSInstaller(spy, testFCOSLogger())

			cfg := &model.InstallConfig{
				OS:       model.OSFCOS,
				Channel:  channel,
				Hostname: "fcos-node",
				Disk:     model.DiskInfo{DevPath: "/dev/vda"},
				Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
				Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
			}

			if err := installer.Install(context.Background(), cfg, func(string) {}); err != nil {
				t.Fatalf("channel %s: unexpected error: %v", channel, err)
			}

			var coreosCall *runner.SpyCall
			for i := range spy.Calls {
				if spy.Calls[i].Name == "coreos-installer" {
					coreosCall = &spy.Calls[i]
					break
				}
			}
			if coreosCall == nil {
				t.Fatal("coreos-installer was not called")
			}
			if !argsContainPair(coreosCall.Args, "--stream", channel) {
				t.Errorf("expected --stream %s in args: %v", channel, coreosCall.Args)
			}
		})
	}
}

func TestFCOSInstall_ExternalIgnitionURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testFCOSLogger())

	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "stable",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}

	if err := installer.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var coreosCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			coreosCall = &spy.Calls[i]
			break
		}
	}
	if coreosCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	if !argsContainPair(coreosCall.Args, "--ignition-url", "https://example.com/fcos.ign") {
		t.Errorf("expected --ignition-url in args: %v", coreosCall.Args)
	}

	// --ignition-file must NOT be present
	for _, arg := range coreosCall.Args {
		if arg == "--ignition-file" {
			t.Error("--ignition-file must not be set when IgnitionURL is provided")
		}
	}
}

func TestFCOSInstall_NilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testFCOSLogger())

	err := installer.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if err.Error() != "install config cannot be nil" {
		t.Errorf("error = %q, want 'install config cannot be nil'", err.Error())
	}
}

func TestFCOSInstall_VersionWarningIgnored(t *testing.T) {
	// Version pinning is not supported for FCOS; the field should be silently ignored
	// (with a log warning). The install must still succeed.
	spy := runner.NewSpyRunner()

	var warnMessages []string
	handler := &capturingHandler{wrapped: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}), warns: &warnMessages}
	logger := slog.New(handler)
	installer := NewFCOSInstaller(spy, logger)

	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Version:  "38.20230514.3.0",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	if err := installer.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Version must NOT appear as --image-url or any version flag
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			for _, arg := range spy.Calls[i].Args {
				if strings.Contains(arg, "38.20230514.3.0") {
					t.Errorf("version string must not appear in coreos-installer args: %v", spy.Calls[i].Args)
				}
			}
		}
	}
}

func TestFCOSInstall_DiskByIDPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testFCOSLogger())

	byIDPath := "/dev/disk/by-id/ata-Samsung_SSD_870_EVO_S6ENNX0T123456"
	cfg := &model.InstallConfig{
		OS:      model.OSFCOS,
		Channel: "stable",
		Disk: model.DiskInfo{
			Path:    byIDPath,
			DevPath: "/dev/sda",
		},
		Hostname: "fcos-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	if err := installer.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			last := spy.Calls[i].Args[len(spy.Calls[i].Args)-1]
			if last != byIDPath {
				t.Errorf("expected by-id path %q as last arg, got %q", byIDPath, last)
			}
			return
		}
	}
	t.Fatal("coreos-installer was not called")
}

func TestBuildFCOSInstallArgs_Basic(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
	}
	args := buildFCOSInstallArgs(cfg, "/tmp/ign.json")

	if args[0] != "install" {
		t.Errorf("first arg must be 'install', got %q", args[0])
	}
	if !argsContainPair(args, "--stream", "stable") {
		t.Errorf("expected --stream stable: %v", args)
	}
	if !argsContainPair(args, "--ignition-file", "/tmp/ign.json") {
		t.Errorf("expected --ignition-file /tmp/ign.json: %v", args)
	}
	if args[len(args)-1] != "/dev/sda" {
		t.Errorf("disk must be last arg, got %q", args[len(args)-1])
	}
}

func TestBuildFCOSInstallArgs_IgnitionURL(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel:     "testing",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/config.ign",
	}
	args := buildFCOSInstallArgs(cfg, "")

	if !argsContainPair(args, "--ignition-url", "https://example.com/config.ign") {
		t.Errorf("expected --ignition-url: %v", args)
	}
	for _, a := range args {
		if a == "--ignition-file" {
			t.Error("--ignition-file must not appear when IgnitionURL is set")
		}
	}
}

func TestBuildFCOSInstallArgs_NoIgnition(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "next",
		Disk:    model.DiskInfo{DevPath: "/dev/nvme0n1"},
	}
	args := buildFCOSInstallArgs(cfg, "")

	for _, a := range args {
		if a == "--ignition-file" || a == "--ignition-url" {
			t.Errorf("unexpected ignition flag %q when neither IgnitionURL nor path set", a)
		}
	}
	if args[len(args)-1] != "/dev/nvme0n1" {
		t.Errorf("disk must be last arg, got %q", args[len(args)-1])
	}
}

// Compile-time check: FCOSInstaller must implement the Installer interface.
var _ Installer = (*FCOSInstaller)(nil)

// argsContainPair returns true if args contains the flag immediately followed by value.
func argsContainPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// argIndex returns the index of flag in args, or -1 if not found.
func argIndex(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

// capturingHandler is a slog.Handler that captures Warn-level messages for assertions.
type capturingHandler struct {
	wrapped slog.Handler
	warns   *[]string
}

func (h *capturingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *capturingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn {
		*h.warns = append(*h.warns, r.Message)
	}
	return nil
}

func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *capturingHandler) WithGroup(name string) slog.Handler {
	return h
}
