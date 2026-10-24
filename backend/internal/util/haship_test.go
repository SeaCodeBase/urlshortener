package util

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashIP_SaltedSHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("salt" + "1.2.3.4"))
	want := hex.EncodeToString(sum[:])
	if got := HashIP("salt", "1.2.3.4"); got != want {
		t.Fatalf("HashIP = %q, want %q", got, want)
	}
	if HashIP("other", "1.2.3.4") == want {
		t.Fatal("salt must change the hash")
	}
}
