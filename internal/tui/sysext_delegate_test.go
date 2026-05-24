package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// ── sysextDelegate.Render ────────────────────────────────────────────────────

func newTestList(items []list.Item, delegate list.ItemDelegate) list.Model {
	l := list.New(items, delegate, 80, 20)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}

func TestSysextDelegateRender_SelectedItem_ShowsCheckmark(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return idx == 0 })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", Version: "24.0", Category: "Container", SupportTier: bakery.TierIntegrated}},
		sysextItem{idx: 1, entry: model.SysextEntry{Name: "tailscale", Version: "1.50", Category: "Network", SupportTier: bakery.TierMaintained}},
	}
	l := newTestList(items, d)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !strings.Contains(out, "[✓]") {
		t.Errorf("selected item should show [✓], got: %q", out)
	}
	if !strings.Contains(out, "docker") {
		t.Errorf("should contain extension name 'docker', got: %q", out)
	}
}

func TestSysextDelegateRender_UnselectedItem_ShowsEmptyCheckbox(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "tailscale", Version: "1.50", Category: "Network", SupportTier: bakery.TierMaintained}},
	}
	l := newTestList(items, d)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !strings.Contains(out, "[ ]") {
		t.Errorf("unselected item should show [ ], got: %q", out)
	}
}

func TestSysextDelegateRender_CurrentItem_ShowsCursor(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", Version: "24.0", Category: "Container", SupportTier: bakery.TierIntegrated}},
	}
	l := newTestList(items, d)
	// Index 0 is the current selection by default in a new list

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !strings.Contains(out, "▸") {
		t.Errorf("current item should show cursor ▸, got: %q", out)
	}
}

func TestSysextDelegateRender_NonCurrentItem_NoCursor(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", Version: "24.0", Category: "Container", SupportTier: bakery.TierIntegrated}},
		sysextItem{idx: 1, entry: model.SysextEntry{Name: "tailscale", Version: "1.50", Category: "Network", SupportTier: bakery.TierMaintained}},
	}
	l := newTestList(items, d)
	// list.Model's initial cursor is at 0, so item 1 is non-current

	var buf bytes.Buffer
	d.Render(&buf, l, 1, items[1])
	out := buf.String()

	if strings.Contains(out, "▸") {
		t.Errorf("non-current item should NOT show cursor ▸, got: %q", out)
	}
}

func TestSysextDelegateRender_TierChange_ShowsHeader(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", SupportTier: bakery.TierIntegrated}},
		sysextItem{idx: 1, entry: model.SysextEntry{Name: "tailscale", SupportTier: bakery.TierMaintained}},
	}
	l := newTestList(items, d)

	// Render item at index 1 — different tier from index 0 → should show header
	var buf bytes.Buffer
	d.Render(&buf, l, 1, items[1])
	out := buf.String()

	if !strings.Contains(out, bakery.TierMaintained) {
		t.Errorf("tier change should show tier header containing %q, got: %q", bakery.TierMaintained, out)
	}
}

func TestSysextDelegateRender_FirstItem_ShowsTierHeader(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", SupportTier: bakery.TierIntegrated}},
	}
	l := newTestList(items, d)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !strings.Contains(out, bakery.TierIntegrated) {
		t.Errorf("first item should show tier header containing %q, got: %q", bakery.TierIntegrated, out)
	}
}

func TestSysextDelegateRender_EmptyVersion_OmitsVPrefix(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "custom-ext", Version: "", Category: "Tools", SupportTier: bakery.TierExperimental}},
	}
	l := newTestList(items, d)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	// Should not have "v" prefix with empty version
	if strings.Contains(out, " v ") || strings.Contains(out, " v\n") {
		t.Errorf("empty version should not produce 'v' prefix, got: %q", out)
	}
	if !strings.Contains(out, "custom-ext") {
		t.Errorf("should contain extension name, got: %q", out)
	}
}

func TestSysextDelegateRender_EmptyCategory_FallsBackToOther(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "mystery", Category: "", SupportTier: ""}},
	}
	l := newTestList(items, d)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !strings.Contains(out, "Other") {
		t.Errorf("empty category should fall back to 'Other', got: %q", out)
	}
}

func TestSysextDelegateRender_InvalidListItem_NoOutput(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return false })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", SupportTier: bakery.TierIntegrated}},
	}
	l := newTestList(items, d)

	// Pass a non-sysextItem — should return immediately with no output
	var buf bytes.Buffer
	d.Render(&buf, l, 0, nonSysextItem{})
	if buf.Len() != 0 {
		t.Errorf("invalid list item should produce no output, got: %q", buf.String())
	}
}

// nonSysextItem satisfies list.Item but is not a sysextItem.
type nonSysextItem struct{}

func (n nonSysextItem) FilterValue() string { return "" }
