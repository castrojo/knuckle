package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// sysextItem wraps a SysextEntry for the bubbles/list interface.
// idx is the index into Wizard.State.Sysexts (stable identity for toggle).
type sysextItem struct {
	idx   int
	entry model.SysextEntry
}

func (i sysextItem) FilterValue() string {
	return i.entry.Name + " " + i.entry.Category + " " + i.entry.SupportTier
}

// sysextDelegate renders sysext items with checkboxes and tier coloring.
type sysextDelegate struct {
	isSelected func(idx int) bool
}

func newSysextDelegate(isSelected func(idx int) bool) sysextDelegate {
	return sysextDelegate{isSelected: isSelected}
}

func (d sysextDelegate) Height() int                             { return 2 }
func (d sysextDelegate) Spacing() int                            { return 0 }
func (d sysextDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d sysextDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(sysextItem)
	if !ok {
		return
	}

	// Checkbox state.
	check := "[ ]"
	if d.isSelected(item.idx) {
		check = "[✓]"
	}

	// Tier-colored badge style.
	tierColor := lipgloss.Color("252")
	switch item.entry.SupportTier {
	case bakery.TierIntegrated:
		tierColor = lipgloss.Color("76") // green
	case bakery.TierMaintained:
		tierColor = lipgloss.Color("75") // blue
	case bakery.TierExperimental:
		tierColor = lipgloss.Color("214") // yellow
	}
	tierStyle := lipgloss.NewStyle().Foreground(tierColor)

	// Build version + category + tier description.
	version := item.entry.Version
	if version != "" {
		version = "v" + version
	}
	cat := item.entry.Category
	if cat == "" {
		cat = "Other"
	}

	// Check if we need a tier section header (tier changed from previous item).
	var header string
	items := m.VisibleItems()
	if index == 0 {
		header = renderTierHeader(item.entry.SupportTier)
	} else if index > 0 && index < len(items) {
		if prev, ok := items[index-1].(sysextItem); ok {
			if prev.entry.SupportTier != item.entry.SupportTier {
				header = renderTierHeader(item.entry.SupportTier)
			}
		}
	}

	// Title line: cursor + checkbox + name + version + category
	title := fmt.Sprintf("%s %-22s %-14s  %s", check, item.entry.Name, version, cat)
	// Description line: tier
	desc := item.entry.SupportTier
	if desc == "" {
		desc = "Other"
	}

	isCurrent := index == m.Index()

	if header != "" {
		_, _ = fmt.Fprint(w, header)
	}

	if isCurrent {
		_, _ = fmt.Fprintf(w, "  ▸ %s\n", selectedStyle.Render(title))
		_, _ = fmt.Fprintf(w, "      %s\n", tierStyle.Render(desc))
	} else {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		_, _ = fmt.Fprintf(w, "    %s\n", dimStyle.Render(title))
		_, _ = fmt.Fprintf(w, "      %s\n", tierStyle.Render(desc))
	}
}

func renderTierHeader(tier string) string {
	if tier == "" {
		tier = "Other"
	}
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	return "  " + dimStyle.Render("── "+tier+" ──") + "\n"
}

// initSysextList builds and configures the bubbles/list model for sysext selection.
func (m *Model) initSysextList() {
	sysexts := m.Wizard.State.Sysexts
	if len(sysexts) == 0 {
		m.sysextListReady = false
		return
	}

	// Build items sorted by tier (matching original order).
	tierOrder := []string{bakery.TierIntegrated, bakery.TierMaintained, bakery.TierExperimental, ""}
	tierMap := map[string][]int{}
	for i, ext := range sysexts {
		tierMap[ext.SupportTier] = append(tierMap[ext.SupportTier], i)
	}

	var items []list.Item
	for _, tier := range tierOrder {
		for _, idx := range tierMap[tier] {
			items = append(items, sysextItem{idx: idx, entry: sysexts[idx]})
		}
	}

	delegate := newSysextDelegate(func(idx int) bool {
		return m.Wizard.State.Sysexts[idx].Selected
	})

	height := m.height - 10
	if height < 10 {
		height = 20
	}
	width := m.width
	if width == 0 {
		width = 80
	}

	l := list.New(items, delegate, width, height)
	l.Title = m.sysextTitle()
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).MarginLeft(2)

	// Disable keys that conflict with parent model.
	l.KeyMap.ForceQuit.SetEnabled(false)

	m.sysextList = l
	m.sysextListReady = true

	// Sync cursor position.
	if m.cursor > 0 && m.cursor < len(items) {
		m.sysextList.Select(m.cursor)
	}
}

// sysextTitle returns the title string with selected count.
func (m *Model) sysextTitle() string {
	selectedCount := 0
	for _, ext := range m.Wizard.State.Sysexts {
		if ext.Selected {
			selectedCount++
		}
	}
	return fmt.Sprintf("System Extensions — %d selected", selectedCount)
}

// sysextListCursorIdx returns the Wizard.State.Sysexts index for the currently
// highlighted item in the list, accounting for filtering.
func (m *Model) sysextListCursorIdx() int {
	if !m.sysextListReady {
		return m.cursor
	}
	item, ok := m.sysextList.SelectedItem().(sysextItem)
	if !ok {
		return m.cursor
	}
	return item.idx
}

// buildSysextItemsFromState rebuilds the list items to reflect current state
// (e.g., after toggling). This preserves the cursor position.
func (m *Model) refreshSysextListTitle() {
	if m.sysextListReady {
		m.sysextList.Title = m.sysextTitle()
	}
}

// sysextListLookup returns the list-internal index for a given Sysexts[] index.
func (m *Model) sysextListLookup(sysextIdx int) int {
	for i, item := range m.sysextList.Items() {
		if si, ok := item.(sysextItem); ok && si.idx == sysextIdx {
			return i
		}
	}
	return 0
}
