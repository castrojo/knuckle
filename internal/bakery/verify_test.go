package bakery

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func TestVerifyFlatcarSignature_ValidSignature(t *testing.T) {
	data, err := os.ReadFile("testdata/flatcar_sbom.DIGESTS.asc")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if !verifyFlatcarSignature(string(data)) {
		t.Error("expected valid signature to verify successfully")
	}
}

func TestVerifyFlatcarSignature_Tampered(t *testing.T) {
	data, err := os.ReadFile("testdata/flatcar_sbom.DIGESTS.asc")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	// Flip one hex digit in the SHA512 hash — body matches but content changed
	tampered := strings.Replace(string(data), "06d849e6", "06d849e7", 1)
	if verifyFlatcarSignature(tampered) {
		t.Error("tampered message should not verify")
	}
}

func TestVerifyFlatcarSignature_NotPGP(t *testing.T) {
	if verifyFlatcarSignature("not a pgp message") {
		t.Error("garbage input should not verify")
	}
}

func TestVerifyFlatcarSignature_Empty(t *testing.T) {
	if verifyFlatcarSignature("") {
		t.Error("empty input should not verify")
	}
}

// TestVerifyFlatcarSignature_KeyringReadError covers the return-false branch
// when readArmoredKeyRingFn returns an error (structurally unreachable in
// production because flatcarSigningKeyASC is a compile-time //go:embed constant).
func TestVerifyFlatcarSignature_KeyringReadError(t *testing.T) {
	old := readArmoredKeyRingFn
	readArmoredKeyRingFn = func(_ io.Reader) (openpgp.EntityList, error) {
		return nil, fmt.Errorf("injected keyring error")
	}
	t.Cleanup(func() { readArmoredKeyRingFn = old })

	if verifyFlatcarSignature("anything") {
		t.Error("keyring read error should return false")
	}
}
