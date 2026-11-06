package runtimegen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const abiFormat = "kgabi1"

// AbiPlaceholder is what the emitted module carries until the fingerprint is
// bound; it has the real one's exact length.
var AbiPlaceholder = abiFormat + ":" + strings.Repeat("0", 32)

// AbiFingerprint hashes what a runtime object and the code emitted against it
// must agree on: the canonical op registry text, the layout, the revision and the flags.
func AbiFingerprint(canonical, layout string, revision int, flags []string) string {
	flags = slices.Compact(slices.Sorted(slices.Values(flags)))
	h := sha256.New()
	fmt.Fprintf(h, "%s\nrevision %d\n%s\nlayout\n%s\nflags %s\n", abiFormat, revision, canonical, layout, strings.Join(flags, " "))
	return abiFormat + ":" + hex.EncodeToString(h.Sum(nil))[:32]
}
