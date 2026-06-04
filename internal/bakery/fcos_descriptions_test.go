package bakery

import "testing"

func TestFCOSLookup_KnownExtensions(t *testing.T) {
	known := []string{
		"docker-ce", "tailscale", "vscode", "vscodium",
		"cilium-cli", "1password-cli",
	}
	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			meta, ok := FCOSLookup(name)
			if !ok {
				t.Fatalf("FCOSLookup(%q) returned ok=false", name)
			}
			if meta.Short == "" {
				t.Error("Short description is empty")
			}
			if meta.Category == "" {
				t.Error("Category is empty")
			}
			if meta.SupportTier == "" {
				t.Error("SupportTier is empty")
			}
		})
	}
}

func TestFCOSLookup_Unknown(t *testing.T) {
	_, ok := FCOSLookup("nonexistent-extension")
	if ok {
		t.Fatal("expected ok=false for unknown extension")
	}
}

func TestFCOSCatalog_MinimumSize(t *testing.T) {
	if len(fcosCatalog) < 6 {
		t.Errorf("fcosCatalog has %d entries, want at least 6", len(fcosCatalog))
	}
}
