package llgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"

	"kigumi/internal/mir"
)

const abiFormat = "kgabi1"

// AbiPlaceholder is what the emitted module carries until BindAbi swaps in
// the real fingerprint; it has the real one's exact length.
var AbiPlaceholder = abiFormat + ":" + strings.Repeat("0", 32)

// AbiFingerprint hashes everything a runtime object and the code emitted
// against it must agree on: the op registry, the value layout, the ABI revision (the driver's) and the runtime flag set.
func AbiFingerprint(reg *mir.StdOpRegistry, layout string, revision int, flags []string) string {
	flags = slices.Compact(slices.Sorted(slices.Values(flags)))
	h := sha256.New()
	fmt.Fprintf(h, "%s\nrevision %d\n%s\nlayout\n%s\nflags %s\n", abiFormat, revision, reg.Canonical(), layout, strings.Join(flags, " "))
	return abiFormat + ":" + hex.EncodeToString(h.Sum(nil))[:32]
}

// CurrentAbiFingerprint is this compiler's fingerprint for a runtime built with flags.
func CurrentAbiFingerprint(revision int, flags []string) string {
	return AbiFingerprint(mir.Registry, currentLayout(), revision, flags)
}

var currentLayout = sync.OnceValue(func() string {
	layout, err := RuntimeLayout()
	if err != nil {
		panic(err)
	}
	return layout
})

// BindAbi swaps the placeholder in o's IR for fp, which the runtime must report at startup.
func (o Output) BindAbi(fp string) Output {
	if len(fp) != len(AbiPlaceholder) || !strings.Contains(o.IR, AbiPlaceholder) {
		panic("llgen: BindAbi on an output with no ABI check, or a fingerprint of the wrong length")
	}
	o.IR = strings.Replace(o.IR, AbiPlaceholder, fp, 1)
	return o
}
