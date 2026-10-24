package util

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashIP is the hex SHA-256 of salt+ip, the visitor hash stored with clicks
// and page views (the same salt makes them comparable).
func HashIP(salt, ip string) string {
	sum := sha256.Sum256([]byte(salt + ip))
	return hex.EncodeToString(sum[:])
}
