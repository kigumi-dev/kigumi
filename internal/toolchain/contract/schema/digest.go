package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// digest hashes v's JSON encoding; struct field order is fixed and map
// keys sort, so no extra canonicalization is needed.
func digest(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// digestBytes hashes raw bytes (a generated file's text) the same way.
func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
