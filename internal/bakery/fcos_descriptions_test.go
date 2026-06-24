package bakery

import (
	"testing"
)

func TestFCOSLookup_KnownExtensions(t *testing.T) {
	known := []string{
		"docker-ce",
		"tailscale",
		"vscode",
		"vscodium",
		"cilium-cli",
		"1password-gui",
		"kubernetes",
		"netbird",
		"cloud-hypervisor",
		"glab",
	}
	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			meta, ok := FCOSLookup(name)
			if !ok {
				t.Fatalf("FCOSLookup(%q) returned ok=false; expected it to be in fcosCatalog", name)
			}
			if meta.Short == "" {
				t.Errorf("FCOSLookup(%q): Short description is empty", name)
			}
			if meta.Category == "" {
				t.Errorf("FCOSLookup(%q): Category is empty", name)
			}
			if meta.SupportTier == "" {
				t.Errorf("FCOSLookup(%q): SupportTier is empty", name)
			}
			if meta.Long == "" {
				t.Errorf("FCOSLookup(%q): Long description is empty", name)
			}
		})
	}
}

func TestFCOSLookup_UnknownExtension(t *testing.T) {
	meta, ok := FCOSLookup("not-a-real-extension-xyz")
	if ok {
		t.Errorf("expected ok=false for unknown extension, got %+v", meta)
	}
}

func TestFCOSLookup_ReturnsCorrectMetadata(t *testing.T) {
	meta, ok := FCOSLookup("docker-ce")
	if !ok {
		t.Fatal("expected docker-ce to be in fcosCatalog")
	}
	if meta.Category != "Container Runtime" {
		t.Errorf("docker-ce Category: got %q, want %q", meta.Category, "Container Runtime")
	}
	if meta.SupportTier != TierFCOSCommunity {
		t.Errorf("docker-ce SupportTier: got %q, want %q", meta.SupportTier, TierFCOSCommunity)
	}
}

func TestFCOSLookup_CatalogSize(t *testing.T) {
	// Require at least 6 entries per issue spec.
	if len(fcosCatalog) < 6 {
		t.Errorf("fcosCatalog has %d entries; require at least 6 per issue spec", len(fcosCatalog))
	}
}

func TestTierFCOSCommunity(t *testing.T) {
	if TierFCOSCommunity == "" {
		t.Error("TierFCOSCommunity constant must not be empty")
	}
}
