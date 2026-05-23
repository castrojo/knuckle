package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func TestRenderTierHeaderWithName(t *testing.T) {
	out := renderTierHeader("Integrated")
	if !strings.Contains(out, "Integrated") {
		t.Errorf("renderTierHeader should contain tier name, got: %q", out)
	}
	if !strings.Contains(out, "──") {
		t.Error("header should contain dashes for styling")
	}
}

func TestRenderTierHeaderEmptyFallback(t *testing.T) {
	out := renderTierHeader("")
	if !strings.Contains(out, "Other") {
		t.Errorf("renderTierHeader(\"\") should fallback to 'Other', got: %q", out)
	}
	if !strings.Contains(out, "──") {
		t.Error("header should contain dashes for styling")
	}
}

func TestSysextTitleNoSelection(t *testing.T) {
	w := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: false},
		{Name: "wasmtime", Selected: false},
	}
	m := New(w)
	title := m.sysextTitle()
	if !strings.Contains(title, "0 selected") {
		t.Errorf("sysextTitle with no selection should show '0 selected', got: %q", title)
	}
}

func TestSysextTitleWithSelections(t *testing.T) {
	w := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: true},
		{Name: "wasmtime", Selected: false},
		{Name: "tailscale", Selected: true},
	}
	m := New(w)
	title := m.sysextTitle()
	if !strings.Contains(title, "2 selected") {
		t.Errorf("sysextTitle should show '2 selected', got: %q", title)
	}
}

func TestRefreshSysextListTitleNotReady(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.sysextListReady = false
	m.refreshSysextListTitle() // must not panic
}

func TestRefreshSysextListTitleReady(t *testing.T) {
	w := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated, Selected: true},
		{Name: "wasmtime", SupportTier: bakery.TierExperimental, Selected: false},
	}
	m := New(w)
	m.initSysextList()
	if !m.sysextListReady {
		t.Fatal("initSysextList should set sysextListReady=true")
	}
	if !strings.Contains(m.sysextList.Title, "1 selected") {
		t.Errorf("initial title should show '1 selected', got: %q", m.sysextList.Title)
	}
	w.State.Sysexts[1].Selected = true
	m.refreshSysextListTitle()
	if !strings.Contains(m.sysextList.Title, "2 selected") {
		t.Errorf("refreshSysextListTitle should update count to '2 selected', got: %q", m.sysextList.Title)
	}
}
