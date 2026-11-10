package sem

import "kigumi/internal/compiler/syntax"

// Layer is a std package's dependency layer: Foundation/Allocation need no
// OS, Platform needs one, Tool is build-sandbox only.
type Layer uint8

const (
	LayerFoundation Layer = iota
	LayerAllocation
	LayerPlatform
	LayerTool
)

// stdLayer is the package-to-layer table. Seeded random is Foundation;
// its clock helper lives in std/random/clock, which is Platform.
var stdLayer = map[string]Layer{
	"std/prelude":                 LayerFoundation,
	"std/math":                    LayerFoundation,
	"std/ffi":                     LayerFoundation,
	"std/mem":                     LayerFoundation,
	"std/task":                    LayerFoundation,
	"std/test":                    LayerFoundation,
	"std/crypto/subtle":           LayerFoundation,
	"std/crypto/hash":             LayerFoundation,
	"std/crypto/sha256":           LayerAllocation,
	"std/crypto/sha512":           LayerAllocation,
	"std/crypto/sha3":             LayerAllocation,
	"std/crypto/chacha20poly1305": LayerAllocation,
	"std/crypto/x25519":           LayerAllocation,
	"std/crypto/mlkem":            LayerAllocation,
	"std/crypto/hmac":             LayerAllocation,
	"std/crypto/hkdf":             LayerAllocation,
	"std/crypto/asn1":             LayerAllocation,
	"std/crypto/bigint":           LayerAllocation,
	"std/crypto/p256":             LayerAllocation,
	"std/crypto/rsa":              LayerAllocation,
	"std/crypto/x509":             LayerAllocation,
	"std/secret":                  LayerFoundation,
	"std/alloc":                   LayerAllocation,
	"std/arch":                    LayerFoundation,
	"std/atomic":                  LayerFoundation,
	"std/sync":                    LayerFoundation,
	"std/encoding/binary":         LayerAllocation,
	"std/encoding/base64":         LayerAllocation,
	"std/encoding/hex":            LayerAllocation,
	"std/compress/flate":          LayerAllocation,
	"std/compress/gzip":           LayerAllocation,
	"std/compress/zlib":           LayerAllocation,
	"std/fmt":                     LayerAllocation,
	"std/error":                   LayerAllocation,
	"std/array":                   LayerAllocation,
	"std/map":                     LayerAllocation,
	"std/sort":                    LayerAllocation,
	"std/text":                    LayerAllocation,
	"std/json":                    LayerAllocation,
	"std/path":                    LayerAllocation,
	"std/io":                      LayerAllocation,
	"std/cli":                     LayerPlatform,
	"std/ffi/dl":                  LayerPlatform,
	"std/entropy":                 LayerPlatform,
	"std/fs":                      LayerPlatform,
	"std/net/http":                LayerPlatform,
	"std/net":                     LayerPlatform,
	"std/net/tls":                 LayerPlatform,
	"std/os":                      LayerPlatform,
	"std/platform":                LayerPlatform,
	"std/proc":                    LayerPlatform,
	"std/random":                  LayerFoundation,
	"std/random/clock":            LayerPlatform,
	"std/shell":                   LayerPlatform,
	"std/sys":                     LayerPlatform,
	"std/shell/coreutil":          LayerPlatform,
	"std/time":                    LayerPlatform,
	"std/build":                   LayerTool,
}

// Runs before resolveImports binds anything, so the graph is
// order-independent across packages; platformAvailability walks it
// transitively, and this pass also checks layer direction per edge.
func (r *Result) buildStdImportGraph() map[PackageID][]PackageID {
	g := make(map[PackageID][]PackageID, len(r.Packages))
	for i := 1; i < len(r.Packages); i++ {
		id := PackageID(i)
		if !r.Packages[id].Std {
			continue
		}
		fromLayer, hasFrom := stdLayer[r.Packages[id].Path]
		for _, f := range r.Packages[id].Files {
			t := r.tree(f)
			for _, d := range r.topDecls(f) {
				if t.Kind(d) != syntax.ImportDecl {
					continue
				}
				s := importDecl(t, d)
				path := pathText(t, s.Path, "/")
				target, ok := r.pathIndex[path]
				if !ok || !r.Packages[target].Std {
					continue
				}
				g[id] = append(g[id], target)
				if toLayer, hasTo := stdLayer[path]; hasFrom && hasTo && toLayer > fromLayer {
					r.errAt(f, s.Path, cStdLayerDirection, path, r.Packages[id].Path)
				}
			}
		}
	}
	return g
}

func (r *Result) platformBlockedCode() (code Code, ok bool) {
	switch {
	case r.target == Freestanding:
		return cTargetFreestanding, true
	case r.noPlatform:
		return cSysNonePlatform, true
	default:
		return Code{}, false
	}
}

// Reaching a Platform package transitively is as unavailable as importing
// it. std's own packages are exempt: `_bare.kg` /
// `_hosted.kg` selection already prunes those edges.
func (r *Result) platformAvailability(from PackageID, path string) (code Code, offendingPath string, ok bool) {
	if r.Packages[from].Std {
		return Code{}, "", false
	}
	start, exists := r.pathIndex[path]
	if !exists || !r.Packages[start].Std {
		return Code{}, "", false
	}
	blockCode, blocked := r.platformBlockedCode()
	if !blocked {
		return Code{}, "", false
	}
	seen := map[PackageID]bool{start: true}
	queue := []PackageID{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if stdLayer[r.Packages[cur].Path] == LayerPlatform {
			return blockCode, r.Packages[cur].Path, true
		}
		for _, next := range r.stdImportGraph[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return Code{}, "", false
}
