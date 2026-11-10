package driver

import (
	"sort"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
)

// reproduciblePrefixMaps names the directories a C input can come from, so the
// paths a reproducible build embeds do not depend on the cwd, the module's
// location, a local replacement's location or the dependency cache's location.
// The maps go widest first; the C compiler takes the last match, so the
// narrowest directory must end the list.
func reproduciblePrefixMaps(m *Module, buildDir string) []buildconfig.PrefixMap {
	maps := []buildconfig.PrefixMap{{Dir: m.Root, Prefix: "."}}
	names := make([]string, 0, len(m.local.Replaces))
	for name := range m.local.Replaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		maps = append(maps, buildconfig.PrefixMap{Dir: m.local.Replaces[name], Prefix: "/kigumi/local/" + name})
	}
	if mods, err := module.CacheModulesDir(m.compiler); err == nil {
		maps = append(maps, buildconfig.PrefixMap{Dir: mods, Prefix: "/kigumi/mod"})
	}
	return append(maps, buildconfig.PrefixMap{Dir: buildDir, Prefix: "."})
}
