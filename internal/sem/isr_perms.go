package sem

import (
	"slices"
	"strings"

	"kigumi/internal/syntax"
)

// isrScope is one handler's resolved permission set and every node's verdict under it.
type isrScope struct {
	handler EntityID
	// own is true when the handler has its own configuration record rather than the root common one.
	own     bool
	permits map[string]bool
	list    []string
	limit   []string
	safe    map[EntityID]bool
}

// InterruptPermissions is a handler's resolved configuration, for explain --fn.
type InterruptPermissions struct {
	Own       bool
	Permits   []string
	RootLimit []string
	// Reached are the sync/MMIO contract identities the handler's summary reaches, permitted or not.
	Reached []string
	// Unverified marks a summary that rests on code with no reviewed contract.
	Unverified bool
}

func (r *Result) handlerKey(id EntityID) string {
	return r.Packages[r.Entities[id].Pkg].Path + "." + r.Entities[id].Name
}

func (r *Result) interruptConfig() InterruptConfig {
	if r.mod == nil {
		return InterruptConfig{}
	}
	return r.mod.Interrupt
}

func (r *Result) isrKnown(id string) bool {
	c := r.isrContracts()
	return c != nil && c.IsrKnown(id)
}

// resolveIsrPerms narrows the root limit by the handler's own record; report gates the E1091 findings.
func (r *Result) resolveIsrPerms(h EntityID, report bool) (bool, []string, []string) {
	cfg := r.interruptConfig()
	e := &r.Entities[h]
	root := map[string]bool{}
	for _, id := range cfg.Root {
		root[id] = true
		if !r.isrKnown(id) && report {
			r.errAt(e.File, e.Node, cInterruptConfig, e.Name, "root", id, "matches no reviewed contract")
		}
	}
	names, own := cfg.Handlers[r.handlerKey(h)]
	if !own {
		names = cfg.Root
	}
	var out []string
	for _, id := range names {
		switch {
		case !r.isrKnown(id):
			if own && report {
				r.errAt(e.File, e.Node, cInterruptConfig, e.Name, "handler", id, "matches no reviewed contract")
			}
		case !root[id]:
			if report {
				r.errAt(e.File, e.Node, cInterruptConfig, e.Name, "handler", id, "is outside the root limit")
			}
		default:
			out = append(out, id)
		}
	}
	slices.Sort(out)
	out = slices.Compact(out)
	limit := slices.Clone(cfg.Root)
	slices.Sort(limit)
	return own, out, limit
}

func (r *Result) InterruptPermissionsOf(id EntityID) (InterruptPermissions, bool) {
	e := &r.Entities[id]
	if e.Kind != EntFn || r.Fn(id).Export != "interrupt" {
		return InterruptPermissions{}, false
	}
	own, list, limit := r.resolveIsrPerms(id, false)
	s := r.opSummary(id)
	return InterruptPermissions{Own: own, Permits: list, RootLimit: limit, Reached: splitContracts(s.Contracts), Unverified: s.Untrusted}, true
}

// isrScopes resolves every handler's permissions, sharing one verdict map per distinct set.
func (r *Result) isrScopes(handlers, nodes []EntityID, report bool) []*isrScope {
	bySet := map[string]map[EntityID]bool{}
	var out []*isrScope
	for _, h := range handlers {
		own, list, limit := r.resolveIsrPerms(h, report)
		sc := &isrScope{handler: h, own: own, list: list, limit: limit, permits: map[string]bool{}}
		for _, id := range list {
			sc.permits[id] = true
		}
		key := strings.Join(list, ";")
		if bySet[key] == nil {
			bySet[key] = r.inferIsrSafe(nodes, sc)
		}
		sc.safe = bySet[key]
		out = append(out, sc)
	}
	return out
}

// checkInterruptConfigKeys rejects a handler record naming no export(interrupt) function of a package this build loaded.
func (r *Result) checkInterruptConfigKeys(handlers []EntityID) {
	known := map[string]bool{}
	for _, h := range handlers {
		known[r.handlerKey(h)] = true
	}
	loaded := map[string]bool{}
	for _, p := range r.Packages {
		loaded[p.Path] = true
	}
	var keys []string
	for k := range r.interruptConfig().Handlers {
		if i := strings.LastIndexByte(k, '.'); !known[k] && i > 0 && loaded[k[:i]] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return
	}
	slices.Sort(keys)
	f, n, ok := r.interruptConfigSite(handlers)
	if !ok {
		return
	}
	for _, k := range keys {
		r.errAt(f, n, cInterruptConfigHandler, k)
	}
}

// interruptConfigSite is where a configuration finding is reported: the first handler, else the main module's first file.
func (r *Result) interruptConfigSite(handlers []EntityID) (FileID, syntax.NodeID, bool) {
	if len(handlers) > 0 {
		e := &r.Entities[handlers[0]]
		return e.File, e.Node, true
	}
	for _, p := range r.Packages {
		if p.Module == "" && !p.Std && len(p.Files) > 0 {
			f := p.Files[0]
			if p.HasScript {
				f = p.Entry
			}
			return f, r.tree(f).Root, true
		}
	}
	return 0, 0, false
}

// configHelp names the configuration origin and the root limit for a handler diagnostic.
func (sc *isrScope) configHelp(name string) string {
	if !sc.own {
		return "`" + name + "` uses the root common interrupt configuration, which permits " + isrPermText(sc.list)
	}
	return "`" + name + "` uses its own interrupt configuration, which permits " + isrPermText(sc.list) + " within the root limit of " + isrPermText(sc.limit)
}

func isrPermText(ids []string) string {
	if len(ids) == 0 {
		return "no MMIO or synchronization operation"
	}
	return "`" + strings.Join(ids, "`, `") + "`"
}
