package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// ── renderZenChrome branches ──────────────────────────────────────────────────

func TestRenderZenChrome_WelcomeStep(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)
	out := m.renderZenChrome()
	// Welcome step skips the version info bar
	if !strings.Contains(out, "K N U C K L E") {
		t.Errorf("renderZenChrome missing logo: %q", out)
	}
}

func TestRenderZenChrome_NonWelcomeWithChannels(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Channel = "stable"
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "4593.2.0", Kernel: "6.12.81", Systemd: "255.13"},
	}
	m := New(w)
	out := m.renderZenChrome()
	if !strings.Contains(out, "4593.2.0") {
		t.Errorf("renderZenChrome should show version from Channels: %q", out)
	}
	if !strings.Contains(out, "6.12.81") {
		t.Errorf("renderZenChrome should show kernel: %q", out)
	}
}

func TestRenderZenChrome_NonWelcomeNoChannelMatch(t *testing.T) {
	// When no channel info matches cfg.Channel, falls back to channel name only.
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Channel = "beta"
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "4593.2.0"},
	}
	m := New(w)
	out := m.renderZenChrome()
	if !strings.Contains(out, "beta") {
		t.Errorf("renderZenChrome fallback should show channel name: %q", out)
	}
}

func TestRenderZenChrome_WithSystemChecks(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.SystemChecks = []wizard.SystemCheck{
		{Name: "disk", Status: "ok"},
		{Name: "ram", Status: "warn"},
		{Name: "cpu", Status: "fail"},
	}
	m := New(w)
	out := m.renderZenChrome()
	// Output should contain the colored dots — just verify it doesn't panic
	// and produces non-empty output
	if len(out) == 0 {
		t.Error("renderZenChrome with system checks returned empty string")
	}
}

// ── viewChannelCards branches ─────────────────────────────────────────────────

func TestViewChannelCards_SelectedChannel(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	m := New(w)
	m.cursor = 0
	m.osSubView = false // skip OS picker, test channel cards directly
	out := m.viewChannelCards()
	if !strings.Contains(out, "Stable") {
		t.Errorf("viewChannelCards should show 'Stable': %q", out)
	}
}

func TestViewChannelCards_LTSDisplayName(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "lts"
	m := New(w)
	m.cursor = 1 // points at "lts" (index 1 in channelList)
	m.osSubView = false // skip OS picker, test channel cards directly
	out := m.viewChannelCards()
	if !strings.Contains(out, "LTS") {
		t.Errorf("viewChannelCards should show 'LTS' for lts channel: %q", out)
	}
}

func TestViewChannelCards_WithVersionInfo(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "4593.2.0", Kernel: "6.12.81"},
	}
	m := New(w)
	m.cursor = 0
	m.osSubView = false // skip OS picker, test channel cards directly
	out := m.viewChannelCards()
	if !strings.Contains(out, "4593.2.0") {
		t.Errorf("viewChannelCards should show version when channels loaded: %q", out)
	}
}

// ── onFormComplete branches ───────────────────────────────────────────────────

func TestOnFormComplete_WelcomeInvalidChannel(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "nightly" // invalid
	m := New(w)
	m.initForm()
	_ = m.onFormComplete()
	if m.err == nil {
		t.Error("expected err set for invalid channel in onFormComplete")
	}
}

func TestOnFormComplete_WelcomeIgnitionURL(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	m := New(w)
	m.initForm()
	_ = m.onFormComplete()
	// Should jump to StepStorage (skipping network/user)
	if w.State.CurrentStep != model.StepStorage {
		t.Errorf("IgnitionURL path should jump to StepStorage, got %v", w.State.CurrentStep)
	}
}

func TestOnFormComplete_ReviewGoBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Confirmed = false // "Go back"
	m := New(w)
	m.initForm()
	_ = m.onFormComplete()
	// Should go back one step
	if w.State.CurrentStep == model.StepReview {
		t.Error("onFormComplete with Confirmed=false should call Previous()")
	}
}

func TestOnFormComplete_NetworkAdvances(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network.Mode = model.NetworkDHCP
	m := New(w)
	m.networkModeInput = "dhcp"
	m.initForm()
	_ = m.onFormComplete()
	if w.State.CurrentStep != model.StepStorage {
		t.Errorf("network onFormComplete should advance to StepStorage, got %v", w.State.CurrentStep)
	}
}
