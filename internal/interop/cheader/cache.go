package cheader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// formatVersion is folded into the cache key; bump it whenever Generate's
// or the manifest's shape changes, so an old entry never masks a new one.
const formatVersion = 12

// cacheKey hashes each closure member's content, not its path, so the same
// bytes reached through a different checkout still hit.
func cacheKey(closure []SourceFile, zigDigest string, argv []string, abi ABI) string {
	hashes := make([]string, len(closure))
	for i, f := range closure {
		hashes[i] = f.Sha256
	}
	sort.Strings(hashes)
	h := sha256.New()
	fmt.Fprintf(h, "kigumi-cheader-v%d\x00%s\x00%v\x00%v\x00", formatVersion, zigDigest, argv, abi)
	for _, hs := range hashes {
		h.Write([]byte(hs))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
