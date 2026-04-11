package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

type Verifier struct{}

func NewVerifier() *Verifier {
	return &Verifier{}
}

func (v *Verifier) VerifySHA256(filePath, expectedHex string) error {
	if expectedHex == "" {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("compute sha256: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedHex, actual)
	}
	return nil
}
