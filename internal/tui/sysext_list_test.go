package tui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func TestInitSysextList_ZeroOneManyEntries(t *testing.T) {
	tests := []struct {
		name       string
		sysexts    []model.SysextEntry
		cursor     int
		wantReady  bool
		wantTitle  string
		wantOrder  []int
		wantCursor int
	}{
		{
			name:      "zero entries",
			wantReady: false,
		},
		{
			name: "single entry",
			sysexts: []model.SysextEntry{
				{Name: "docker", SupportTier: bakery.TierIntegrated},
			},
			wantReady:  true,
			wantTitle:  "0 selected",
			wantOrder:  []int{0},
			wantCursor: 0,
		},
		{
			name: "many entries sort by tier and keep cursor",
			sysexts: []model.SysextEntry{
				{Name: "other", SupportTier: "", Selected: false},
				{Name: "maintained", SupportTier: bakery.TierMaintained, Selected: true},
				{Name: "integrated", SupportTier: bakery.TierIntegrated, Selected: false},
				{Name: "experimental", SupportTier: bakery.TierExperimental, Selected: true},
			},
			cursor:     2,
			wantReady:  true,
			wantTitle:  "2 selected",
			wantOrder:  []int{2, 1, 3, 0},
			wantCursor: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			w.State.Sysexts = tt.sysexts
			m := New(w)
			m.cursor = tt.cursor
			m.initSysextList()

			if m.sysextListReady != tt.wantReady {
				t.Fatalf("sysextListReady = %v, want %v", m.sysextListReady, tt.wantReady)
			}
			if !tt.wantReady {
				return
			}
			if !strings.Contains(m.sysextList.Title, tt.wantTitle) {
				t.Fatalf("Title = %q, want substring %q", m.sysextList.Title, tt.wantTitle)
			}
			if got := m.sysextList.Index(); got != tt.wantCursor {
				t.Fatalf("Index() = %d, want %d", got, tt.wantCursor)
			}
			items := m.sysextList.Items()
			if len(items) != len(tt.wantOrder) {
				t.Fatalf("len(Items()) = %d, want %d", len(items), len(tt.wantOrder))
			}
			for i, wantIdx := range tt.wantOrder {
				si, ok := items[i].(sysextItem)
				if !ok {
					t.Fatalf("item %d has unexpected type %T", i, items[i])
				}
				if si.idx != wantIdx {
					t.Fatalf("item %d idx = %d, want %d", i, si.idx, wantIdx)
				}
			}
		})
	}
}

func TestSysextDelegate_MethodBehavior(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return idx == 1 })
	if d.Height() != 2 {
		t.Fatalf("Height() = %d, want 2", d.Height())
	}
	if d.Spacing() != 0 {
		t.Fatalf("Spacing() = %d, want 0", d.Spacing())
	}
	if cmd := d.Update(nil, nil); cmd != nil {
		t.Fatal("Update() returned unexpected command")
	}

	item := sysextItem{idx: 1, entry: model.SysextEntry{Name: "tailscale", Category: "Network", SupportTier: bakery.TierMaintained}}
	if got := item.FilterValue(); !strings.Contains(got, "tailscale") || !strings.Contains(got, "Network") || !strings.Contains(got, bakery.TierMaintained) {
		t.Fatalf("FilterValue() = %q, want name/category/tier content", got)
	}
}

func TestSysextDelegateRender_ListScenarios(t *testing.T) {
	d := newSysextDelegate(func(idx int) bool { return idx == 0 })
	items := []list.Item{
		sysextItem{idx: 0, entry: model.SysextEntry{Name: "docker", Version: "24.0", Category: "Container", SupportTier: bakery.TierIntegrated}},
		sysextItem{idx: 1, entry: model.SysextEntry{Name: "tailscale", Version: "1.50", Category: "Network", SupportTier: bakery.TierMaintained}},
		sysextItem{idx: 2, entry: model.SysextEntry{Name: "custom", Category: "", SupportTier: ""}},
	}
	l := newTestList(items, d)

	var first bytes.Buffer
	d.Render(&first, l, 0, items[0])
	if out := first.String(); !strings.Contains(out, "[✓]") || !strings.Contains(out, bakery.TierIntegrated) {
		t.Fatalf("first render missing checkmark or header: %q", out)
	}

	var second bytes.Buffer
	d.Render(&second, l, 1, items[1])
	if out := second.String(); !strings.Contains(out, bakery.TierMaintained) || strings.Contains(out, "▸") {
		t.Fatalf("second render missing tier change header or unexpectedly current: %q", out)
	}

	var third bytes.Buffer
	d.Render(&third, l, 2, items[2])
	if out := third.String(); !strings.Contains(out, "Other") {
		t.Fatalf("third render should fall back to Other, got: %q", out)
	}
	var invalid bytes.Buffer
	d.Render(&invalid, l, 0, nonSysextItem{})
	if invalid.Len() != 0 {
		t.Fatalf("invalid item should render nothing, got %q", invalid.String())
	}
}
