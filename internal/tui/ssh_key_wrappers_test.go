package tui

import (
	"testing"
)

// Tests for the tui-package wrappers splitSSHKeys and mergeKeys.
// These are thin delegates to wizard.SplitSSHKeys / wizard.MergeSSHKeys.
// The tests confirm correct delegation and close the 0% coverage gap.

func TestSplitSSHKeys_SemicolonSeparated(t *testing.T) {
	input := "ssh-ed25519 AAAA1;ssh-ed25519 AAAA2"
	got := splitSSHKeys(input)
	if len(got) != 2 {
		t.Fatalf("splitSSHKeys: got %d keys, want 2", len(got))
	}
	if got[0] != "ssh-ed25519 AAAA1" {
		t.Errorf("splitSSHKeys[0] = %q, want %q", got[0], "ssh-ed25519 AAAA1")
	}
	if got[1] != "ssh-ed25519 AAAA2" {
		t.Errorf("splitSSHKeys[1] = %q, want %q", got[1], "ssh-ed25519 AAAA2")
	}
}

func TestSplitSSHKeys_EmptyInput(t *testing.T) {
	got := splitSSHKeys("")
	if len(got) != 0 {
		t.Errorf("splitSSHKeys(\"\") = %v, want empty", got)
	}
}

func TestSplitSSHKeys_TrimsWhitespace(t *testing.T) {
	input := "  ssh-ed25519 AAAA1  ;  ssh-ed25519 AAAA2  "
	got := splitSSHKeys(input)
	if len(got) != 2 {
		t.Fatalf("splitSSHKeys: got %d keys, want 2", len(got))
	}
	if got[0] != "ssh-ed25519 AAAA1" {
		t.Errorf("splitSSHKeys[0] = %q, want trimmed", got[0])
	}
}

func TestMergeKeys_DeduplicatesAcrossLists(t *testing.T) {
	a := []string{"ssh-ed25519 AAAA1", "ssh-ed25519 AAAA2"}
	b := []string{"ssh-ed25519 AAAA2", "ssh-ed25519 AAAA3"}

	got := mergeKeys(a, b)
	if len(got) != 3 {
		t.Fatalf("mergeKeys: got %d keys, want 3 (deduped)", len(got))
	}
}

func TestMergeKeys_EmptyInputs(t *testing.T) {
	got := mergeKeys(nil, nil)
	if len(got) != 0 {
		t.Errorf("mergeKeys(nil, nil) = %v, want empty", got)
	}
}

func TestMergeKeys_PreservesFirstSeenOrder(t *testing.T) {
	a := []string{"ssh-ed25519 AAAA1"}
	b := []string{"ssh-ed25519 AAAA2"}
	c := []string{"ssh-ed25519 AAAA1"} // duplicate of first

	got := mergeKeys(a, b, c)
	if len(got) != 2 {
		t.Fatalf("mergeKeys: got %d keys, want 2", len(got))
	}
	if got[0] != "ssh-ed25519 AAAA1" {
		t.Errorf("mergeKeys[0] = %q, want first-seen key", got[0])
	}
	if got[1] != "ssh-ed25519 AAAA2" {
		t.Errorf("mergeKeys[1] = %q, want second key", got[1])
	}
}
