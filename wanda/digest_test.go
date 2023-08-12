package wanda

import (
	"testing"
)

func TestSha256Digest(t *testing.T) {
	const msg1 = "hello world"
	const msg2 = "Hello world!"

	d1 := sha256Digest([]byte(msg1))
	d2 := sha256Digest([]byte(msg2))

	if d1 == d2 {
		t.Errorf("got same digest after adding file: %q vs %q", d1, d2)
	}
}
