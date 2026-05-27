package bakery

import (
	"bytes"
	_ "embed"
	"io"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

//go:embed keys/flatcar-signing.asc
var flatcarSigningKeyASC string

// readArmoredKeyRingFn is the function used to parse a PGP armored key ring.
// Tests can replace it to simulate keyring-parse failures.
var readArmoredKeyRingFn = func(r io.Reader) (openpgp.EntityList, error) {
	return openpgp.ReadArmoredKeyRing(r)
}

// verifyFlatcarSignature verifies a cleartext-signed PGP message (the .DIGESTS.asc
// format used by Flatcar releases) against the embedded Flatcar release key.
// Returns true only when the signature is cryptographically valid.
func verifyFlatcarSignature(signedMessage string) bool {
	keyring, err := readArmoredKeyRingFn(strings.NewReader(flatcarSigningKeyASC))
	if err != nil {
		return false
	}

	block, _ := clearsign.Decode([]byte(signedMessage))
	if block == nil {
		return false
	}

	// block.Plaintext is []byte; block.ArmoredSignature is *armor.Block with a Body io.Reader.
	_, err = openpgp.CheckDetachedSignature(keyring,
		bytes.NewReader(block.Plaintext),
		block.ArmoredSignature.Body,
		&packet.Config{},
	)
	return err == nil
}
