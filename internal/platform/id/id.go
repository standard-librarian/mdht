package id

import (
	"crypto/rand"
	"encoding/hex"
)

// Generator creates opaque identifiers.
type Generator interface {
	New() string
}

type RandomHex struct{}

func (RandomHex) New() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
