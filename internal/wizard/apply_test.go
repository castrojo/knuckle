package wizard

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func newApplyWizard() *Wizard {
	return &Wizard{State: &State{Config: model.InstallConfig{}}}
}

func TestApplyNetworkStepStatic(t *testing.T) {
	w := newApplyWizard()
	w.ApplyNetworkStep(NetworkStepInput{Mode: "static", DNS: "1.1.1.1, 8.8.8.8 ,"})
	if w.State.Config.Network.Mode != model.NetworkStatic {
		t.Errorf("mode: got %v want static", w.State.Config.Network.Mode)
	}
	want := []string{"1.1.1.1", "8.8.8.8"}
	got := w.State.Config.Network.DNS
	if len(got) != len(want) {
		t.Fatalf("DNS: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DNS[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestApplyNetworkStepDHCPClearsDNS(t *testing.T) {
	w := newApplyWizard()
	w.State.Config.Network.DNS = []string{"1.1.1.1"}
	w.ApplyNetworkStep(NetworkStepInput{Mode: "dhcp", DNS: ""})
	if w.State.Config.Network.Mode != model.NetworkDHCP {
		t.Errorf("mode: got %v want dhcp", w.State.Config.Network.Mode)
	}
	if len(w.State.Config.Network.DNS) != 0 {
		t.Errorf("DNS should be cleared, got %v", w.State.Config.Network.DNS)
	}
}

func TestApplyUserStepCreatesUserAndHashesPassword(t *testing.T) {
	w := newApplyWizard()
	err := w.ApplyUserStep(UserStepInput{
		Username:  "alice",
		Password:  "hunter2",
		ManualKey: "ssh-ed25519 AAAA alice@laptop",
		LocalKeys: []string{"ssh-ed25519 BBBB alice@host"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	cfg := w.State.Config
	if len(cfg.Users) != 1 || cfg.Users[0].Username != "alice" {
		t.Errorf("expected user alice, got %+v", cfg.Users)
	}
	if cfg.Users[0].PasswordHash == "" {
		t.Error("expected bcrypt password hash")
	}
	if !strings.HasPrefix(cfg.Users[0].PasswordHash, "$2") {
		t.Errorf("expected bcrypt $2 prefix, got %q", cfg.Users[0].PasswordHash)
	}
	wantKeys := []string{"ssh-ed25519 BBBB alice@host", "ssh-ed25519 AAAA alice@laptop"}
	if got := cfg.SSHKeys; len(got) != 2 || got[0] != wantKeys[0] || got[1] != wantKeys[1] {
		t.Errorf("SSH keys: got %v want %v", got, wantKeys)
	}
	groups := cfg.Users[0].Groups
	hasDocker := false
	hasSudo := false
	for _, g := range groups {
		if g == "docker" {
			hasDocker = true
		}
		if g == "sudo" {
			hasSudo = true
		}
	}
	if !hasDocker || !hasSudo {
		t.Errorf("expected default groups sudo+docker, got %v", groups)
	}
}

func TestApplyUserStepRejectsLongPassword(t *testing.T) {
	w := newApplyWizard()
	err := w.ApplyUserStep(UserStepInput{
		Username: "bob",
		Password: strings.Repeat("a", 73),
	})
	if err == nil {
		t.Fatal("expected error for >72 byte password")
	}
}

func TestApplyGitHubKeysDedupes(t *testing.T) {
	w := newApplyWizard()
	w.State.Config.Users = []model.UserConfig{{Username: "carol"}}
	w.ApplyGitHubKeys(
		[]string{"ssh-ed25519 GH carol@github"},
		[]string{"ssh-ed25519 LOCAL carol@host"},
		"ssh-ed25519 LOCAL carol@host;ssh-ed25519 MANUAL carol@laptop",
	)
	cfg := w.State.Config
	if len(cfg.SSHKeys) != 3 {
		t.Errorf("expected 3 deduped keys, got %d: %v", len(cfg.SSHKeys), cfg.SSHKeys)
	}
	if len(cfg.Users[0].SSHKeys) != 3 {
		t.Errorf("user SSH keys should mirror cfg.SSHKeys, got %v", cfg.Users[0].SSHKeys)
	}
}

func TestHasAnyAuthentication(t *testing.T) {
	w := newApplyWizard()
	if w.HasAnyAuthentication() {
		t.Error("empty config should have no auth")
	}
	w.State.Config.Users = []model.UserConfig{{Username: "x"}}
	if w.HasAnyAuthentication() {
		t.Error("user without password/keys should have no auth")
	}
	w.State.Config.Users[0].PasswordHash = "hash"
	if !w.HasAnyAuthentication() {
		t.Error("password-only should count as auth")
	}
	w.State.Config = model.InstallConfig{SSHKeys: []string{"ssh-x"}}
	if !w.HasAnyAuthentication() {
		t.Error("SSH-key-only should count as auth")
	}
}

func TestSplitSSHKeys(t *testing.T) {
	got := SplitSSHKeys("  ssh-rsa AAA  ;  ;ssh-ed25519 BBB ; ")
	want := []string{"ssh-rsa AAA", "ssh-ed25519 BBB"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SplitSSHKeys: got %v want %v", got, want)
	}
}

func TestMergeSSHKeysPreservesOrder(t *testing.T) {
	got := MergeSSHKeys(
		[]string{"a", "b"},
		[]string{"b", "c"},
		[]string{"a", "d"},
	)
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestMergeSSHKeys_FiltersEmptyStrings(t *testing.T) {
	// The k == "" → continue branch is only hit when an empty string appears in a list.
	result := MergeSSHKeys([]string{"ssh-ed25519 AAAA key1", "", "ssh-ed25519 AAAA key2"})
	if len(result) != 2 {
		t.Errorf("expected 2 keys (empty string filtered), got %d: %v", len(result), result)
	}
	for _, k := range result {
		if k == "" {
			t.Error("empty string should be filtered out by MergeSSHKeys")
		}
	}
}

// ── HashPassword direct tests ────────────────────────────────────────────────

func TestHashPassword_ValidPassword_ReturnsBcryptHash(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("expected bcrypt hash prefix, got: %q", hash[:10])
	}
}

func TestHashPassword_EmptyPassword_Succeeds(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("empty password should succeed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash for empty password")
	}
}

func TestHashPassword_Exactly72Bytes_Succeeds(t *testing.T) {
	pw := strings.Repeat("x", 72)
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("72-byte password should succeed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashPassword_73Bytes_ReturnsError(t *testing.T) {
	pw := strings.Repeat("x", 73)
	_, err := HashPassword(pw)
	if err == nil {
		t.Fatal("73-byte password should fail")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("error should mention 'too long', got: %v", err)
	}
}

func TestHashPassword_MultiByte_CountsBytes(t *testing.T) {
	// 24 runes of 3-byte chars = 72 bytes exactly → should succeed
	pw := strings.Repeat("日", 24) // 24 * 3 = 72 bytes
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("72-byte multibyte password should succeed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// 25 runes = 75 bytes → should fail
	pw = strings.Repeat("日", 25)
	_, err = HashPassword(pw)
	if err == nil {
		t.Fatal("75-byte multibyte password should fail")
	}
}
