package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/probe"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// ── viewNvidia ───────────────────────────────────────────────────────────────

func TestViewNvidia_GPUDetected_ShowsGPUInfo(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{
		{PCIAddress: "0000:01:00.0", DeviceName: "GA102 [GeForce RTX 3080]"},
	}
	m := New(w)
	m.cursor = 0

	out := m.viewNvidia()

	if !strings.Contains(out, "NVIDIA GPU detected") {
		t.Errorf("expected detected message, got: %q", out)
	}
	if !strings.Contains(out, "0000:01:00.0") {
		t.Errorf("expected PCI address in output, got: %q", out)
	}
	if !strings.Contains(out, "GeForce RTX 3080") {
		t.Errorf("expected device name in output, got: %q", out)
	}
}

func TestViewNvidia_NoGPU_ShowsWarning(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = false
	w.State.NvidiaGPUs = nil
	m := New(w)

	out := m.viewNvidia()

	if !strings.Contains(out, "No NVIDIA GPU detected") {
		t.Errorf("expected no-GPU warning, got: %q", out)
	}
	if strings.Contains(out, "0000:") {
		t.Errorf("should not show PCI address when no GPU detected, got: %q", out)
	}
}

func TestViewNvidia_CursorRendersSelectedDriver(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{
		{PCIAddress: "0000:01:00.0", DeviceName: "Test GPU"},
	}
	m := New(w)

	// The first option should be selected with cursor at 0
	m.cursor = 0
	out := m.viewNvidia()

	if len(model.NvidiaDriverOptions) == 0 {
		t.Skip("NvidiaDriverOptions is empty")
	}
	firstLabel := model.NvidiaDriverOptions[0].Label
	if !strings.Contains(out, firstLabel) {
		t.Errorf("expected first driver label %q in output, got: %q", firstLabel, out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("expected cursor indicator '▸' in output, got: %q", out)
	}
}

func TestViewNvidia_RecommendedTagRendered(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{
		{PCIAddress: "0000:01:00.0", DeviceName: "Test GPU"},
	}
	m := New(w)

	// Find the recommended option index
	recommIdx := -1
	for i, opt := range model.NvidiaDriverOptions {
		if opt.Recommended {
			recommIdx = i
			break
		}
	}
	if recommIdx < 0 {
		t.Skip("no recommended option in NvidiaDriverOptions")
	}

	m.cursor = recommIdx
	out := m.viewNvidia()

	if !strings.Contains(out, "[recommended]") {
		t.Errorf("expected [recommended] tag when cursor is on recommended option, got: %q", out)
	}
}

func TestViewNvidia_MultipleGPUs_ShowsAll(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{
		{PCIAddress: "0000:01:00.0", DeviceName: "RTX 4090"},
		{PCIAddress: "0000:02:00.0", DeviceName: "RTX 3070"},
	}
	m := New(w)

	out := m.viewNvidia()

	if !strings.Contains(out, "RTX 4090") {
		t.Errorf("expected first GPU name, got: %q", out)
	}
	if !strings.Contains(out, "RTX 3070") {
		t.Errorf("expected second GPU name, got: %q", out)
	}
	if !strings.Contains(out, "0000:02:00.0") {
		t.Errorf("expected second PCI address, got: %q", out)
	}
}

func TestViewNvidia_DescriptionShownForSelectedOption(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = false
	m := New(w)

	// Find an option with a description
	descIdx := -1
	var desc string
	for i, opt := range model.NvidiaDriverOptions {
		if opt.Description != "" {
			descIdx = i
			desc = opt.Description
			break
		}
	}
	if descIdx < 0 {
		t.Skip("no option with description")
	}

	m.cursor = descIdx
	out := m.viewNvidia()

	if !strings.Contains(out, desc) {
		t.Errorf("expected description %q for selected option, got: %q", desc, out)
	}
}

func TestViewNvidia_ContainsKernelDriverExplanation(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{
		{PCIAddress: "0000:01:00.0", DeviceName: "Test"},
	}
	m := New(w)

	out := m.viewNvidia()

	if !strings.Contains(out, "kernel driver") {
		t.Errorf("expected kernel driver explanation, got: %q", out)
	}
	if !strings.Contains(out, "Container Toolkit") {
		t.Errorf("expected Container Toolkit explanation, got: %q", out)
	}
}
