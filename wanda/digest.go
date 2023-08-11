package wanda

import (
	"crypto/sha256"
	"fmt"
	"hash"
)

func sha256DigestString(h hash.Hash) string {
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func sha256Digest(bs []byte) string {
	h := sha256.New()
	h.Write(bs)
	return sha256DigestString(h)
}
